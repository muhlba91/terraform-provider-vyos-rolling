package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/echowings/terraform-provider-vyos-rolling/internal/client/clienterrors"
	"github.com/echowings/terraform-provider-vyos-rolling/internal/terraform/helpers/tools"
)

// TODO create a client request internal function
//  deduplicate the work being done in the public functions by
//  having a private function to send the request and receive the response
//  milestone: 6

// NewClient creates a new client object to use with VyOS CRUD functions
func NewClient(
	ctx context.Context,
	endpoint string,
	apiKey string,
	userAgent string,
	disableVerify bool,
) *Client {
	return newClient(ctx, endpoint, apiKey, userAgent, disableVerify, 0)
}

// NewClientWithRetries is an overload of NewClient that allows configuring HTTP retry attempts.
func NewClientWithRetries(
	ctx context.Context,
	endpoint string,
	apiKey string,
	userAgent string,
	disableVerify bool,
	retryAttempts int,
) *Client {
	return newClient(ctx, endpoint, apiKey, userAgent, disableVerify, retryAttempts)
}

func newClient(
	ctx context.Context,
	endpoint string,
	apiKey string,
	userAgent string,
	disableVerify bool,
	retryAttempts int,
) *Client {
	if retryAttempts < 0 {
		retryAttempts = 0
	}

	c := &Client{
		httpClient: http.Client{},

		userAgent: userAgent,
		endpoint:  endpoint,
		apiKey:    apiKey,

		state:       newCommitState(),
		batchWindow: 500 * time.Millisecond,

		requestRetryAttempts: retryAttempts,
	}

	if disableVerify {
		tools.Warn(ctx, "Disabling TLS Certificate Verification")
		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	return c
}

// SetBindingOverrides configures manual binding rules where the map key is the
// VyOS path prefix (joined by spaces) and the value is the binding key that
// should be shared by matching resources.
func (c *Client) SetBindingOverrides(overrides map[string]string) {
	c.bindingMu.Lock()
	defer c.bindingMu.Unlock()

	c.bindingOverrides = c.bindingOverrides[:0]
	for prefix, target := range overrides {
		prefix = strings.TrimSpace(prefix)
		target = strings.TrimSpace(target)
		if prefix == "" || target == "" {
			continue
		}
		c.bindingOverrides = append(c.bindingOverrides, bindingOverride{
			prefix: prefix,
			bindAs: target,
		})
	}

	sort.SliceStable(c.bindingOverrides, func(i, j int) bool {
		return len(c.bindingOverrides[i].prefix) > len(c.bindingOverrides[j].prefix)
	})
}

// Client wrapper around http client with convenience functions
// Use NewClient() to generate a new client
type Client struct {
	httpClient http.Client

	userAgent string
	endpoint  string
	apiKey    string

	state       *commitState
	commitMu    sync.Mutex
	batchWindow time.Duration

	requestRetryAttempts int

	bindingMu        sync.RWMutex
	bindingOverrides []bindingOverride
}

type resourceBatch struct {
	resourceID string
	bindingKey string
	setOps     [][]string
	deleteOps  [][]string
}

type bindingGroup struct {
	key       string
	resources []*resourceBatch
}

type resourceResult struct {
	data any
	err  error
}

type bindingOverride struct {
	prefix string
	bindAs string
}

type commitState struct {
	mu        sync.Mutex
	pending   map[string]*resourceBatch
	order     []string
	completed map[string]resourceResult
}

const (
	gatewayRetryInitialDelay = 500 * time.Millisecond
	gatewayRetryMultiplier   = 1.5
	gatewayRetryMaxDelay     = 5 * time.Second

	httpRetryInitialDelay = 500 * time.Millisecond
	httpRetryMultiplier   = 2.0
	httpRetryMaxDelay     = 5 * time.Second
)

func newCommitState() *commitState {
	return &commitState{
		pending:   make(map[string]*resourceBatch),
		completed: make(map[string]resourceResult),
	}
}

func (s *commitState) ensureBatchLocked(resourceID, bindingKey string) *resourceBatch {
	if batch, ok := s.pending[resourceID]; ok {
		return batch
	}

	batch := &resourceBatch{resourceID: resourceID, bindingKey: bindingKey}
	s.pending[resourceID] = batch
	s.order = append(s.order, resourceID)
	return batch
}

func (s *commitState) addSet(resourceID, bindingKey string, ops [][]string) {
	if len(ops) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	batch := s.ensureBatchLocked(resourceID, bindingKey)
	batch.setOps = append(batch.setOps, ops...)
}

func (s *commitState) addDelete(resourceID, bindingKey string, ops [][]string) {
	if len(ops) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	batch := s.ensureBatchLocked(resourceID, bindingKey)
	batch.deleteOps = append(batch.deleteOps, ops...)
}

func (s *commitState) drainPending() []*resourceBatch {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.order) == 0 {
		return nil
	}
	result := make([]*resourceBatch, 0, len(s.order))
	for _, resourceID := range s.order {
		result = append(result, s.pending[resourceID])
	}
	s.pending = make(map[string]*resourceBatch)
	s.order = nil
	return result
}

func (s *commitState) hasPending(resourceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.pending[resourceID]
	return ok
}

func (s *commitState) storeCompleted(results map[string]resourceResult) {
	if len(results) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for resourceID, res := range results {
		s.completed[resourceID] = res
	}
}

func (s *commitState) popCompleted(resourceID string) (resourceResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, ok := s.completed[resourceID]
	if ok {
		delete(s.completed, resourceID)
	}
	return res, ok
}

// StageSet saves vyos paths to configure during commit for a specific resource
func (c *Client) StageSet(ctx context.Context, resourcePath []string, values [][]string) {
	resourceID := c.resourceKey(resourcePath)
	bindingKey := c.bindingKey(resourcePath)
	tools.Trace(ctx, "stageing set ops", map[string]interface{}{"client:httpClient": fmt.Sprintf("%p:%p", c, &c.httpClient), "paths": values, "resource": resourceID, "binding": bindingKey})
	c.state.addSet(resourceID, bindingKey, values)
}

// StageDelete saves vyos paths to delete during commit for a specific resource
func (c *Client) StageDelete(ctx context.Context, resourcePath []string, values [][]string) {
	resourceID := c.resourceKey(resourcePath)
	bindingKey := c.bindingKey(resourcePath)
	tools.Trace(ctx, "stageing delete ops", map[string]interface{}{"client:httpClient": fmt.Sprintf("%p:%p", c, &c.httpClient), "paths": values, "resource": resourceID, "binding": bindingKey})
	c.state.addDelete(resourceID, bindingKey, values)
}

// CommitChanges executes staged vyos paths for the provided resource path.
// Order of operations as they are sent to VyOS:
//  1. delete
//  2. set
func (c *Client) CommitChanges(ctx context.Context, resourcePath []string) (any, error) {
	resourceID := c.resourceKey(resourcePath)

	if res, ok := c.state.popCompleted(resourceID); ok {
		return res.data, res.err
	}

	if !c.state.hasPending(resourceID) {
		return nil, nil
	}

	c.commitMu.Lock()
	defer c.commitMu.Unlock()

	if res, ok := c.state.popCompleted(resourceID); ok {
		return res.data, res.err
	}

	if !c.state.hasPending(resourceID) {
		return nil, nil
	}

	if err := c.waitForBatchWindow(ctx); err != nil {
		return nil, err
	}

	batches := c.state.drainPending()
	if len(batches) == 0 {
		return nil, nil
	}

	results := c.processBatches(ctx, batches)
	c.state.storeCompleted(results)

	if res, ok := results[resourceID]; ok {
		return res.data, res.err
	}

	if res, ok := c.state.popCompleted(resourceID); ok {
		return res.data, res.err
	}

	return nil, fmt.Errorf("missing commit result for resource '%s'", resourceID)
}

func (c *Client) waitForBatchWindow(ctx context.Context) error {
	if c.batchWindow <= 0 {
		return nil
	}

	timer := time.NewTimer(c.batchWindow)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) processBatches(ctx context.Context, batches []*resourceBatch) map[string]resourceResult {
	results := make(map[string]resourceResult, len(batches))
	groups := groupBatches(batches)
	operations := buildOperations(groups)

	data, err := c.sendOperations(ctx, operations)
	if err == nil {
		for _, grp := range groups {
			res := resourceResult{data: data, err: nil}
			for _, batch := range grp.resources {
				results[batch.resourceID] = res
			}
		}
		return results
	}

	tools.Warn(ctx, "Batched commit failed, retrying per binding group", map[string]interface{}{"error": err})
	for _, grp := range groups {
		ops := buildOperations([]*bindingGroup{grp})
		data, groupErr := c.sendOperations(ctx, ops)
		if groupErr != nil {
			groupErr = fmt.Errorf("commit failed for binding '%s': %w", grp.key, groupErr)
		}
		res := resourceResult{data: data, err: groupErr}
		for _, batch := range grp.resources {
			results[batch.resourceID] = res
		}
	}

	return results
}

func groupBatches(batches []*resourceBatch) []*bindingGroup {
	if len(batches) == 0 {
		return nil
	}

	groups := make(map[string]*bindingGroup)
	order := make([]string, 0, len(batches))
	for _, batch := range batches {
		key := batch.bindingKey
		if key == "" {
			key = batch.resourceID
		}
		grp, ok := groups[key]
		if !ok {
			grp = &bindingGroup{key: key}
			groups[key] = grp
			order = append(order, key)
		}
		grp.resources = append(grp.resources, batch)
	}

	result := make([]*bindingGroup, 0, len(order))
	for _, key := range order {
		result = append(result, groups[key])
	}

	return result
}

func buildOperations(groups []*bindingGroup) []map[string]interface{} {
	if len(groups) == 0 {
		return nil
	}

	ops := make([]map[string]interface{}, 0)
	for _, grp := range groups {
		for _, batch := range grp.resources {
			for _, path := range batch.deleteOps {
				ops = append(ops, map[string]interface{}{"op": "delete", "path": path})
			}
		}
	}
	for _, grp := range groups {
		for _, path := range prioritizedSetOps(grp) {
			ops = append(ops, map[string]interface{}{"op": "set", "path": path})
		}
	}

	return ops
}

func prioritizedSetOps(grp *bindingGroup) [][]string {
	if grp == nil || len(grp.resources) == 0 {
		return nil
	}

	type weightedPath struct {
		path   []string
		weight int
		order  int
	}

	paths := make([]weightedPath, 0)
	nextOrder := 0
	for _, batch := range grp.resources {
		for _, path := range batch.setOps {
			paths = append(paths, weightedPath{
				path:   path,
				weight: firewallZoneSetWeight(path),
				order:  nextOrder,
			})
			nextOrder++
		}
	}

	sort.SliceStable(paths, func(i, j int) bool {
		if paths[i].weight == paths[j].weight {
			return paths[i].order < paths[j].order
		}
		return paths[i].weight < paths[j].weight
	})

	result := make([][]string, 0, len(paths))
	for _, entry := range paths {
		result = append(result, entry.path)
	}

	return result
}

func firewallZoneSetWeight(path []string) int {
	if isFirewallZoneMemberInterface(path) {
		return 100
	}
	return 0
}

func isFirewallZoneMemberInterface(path []string) bool {
	if len(path) < 6 {
		return false
	}
	if path[0] != "firewall" || path[1] != "zone" {
		return false
	}
	for i := 2; i < len(path)-1; i++ {
		if path[i] == "member" && path[i+1] == "interface" {
			return true
		}
	}
	return false
}

func (c *Client) sendOperations(ctx context.Context, operations []map[string]interface{}) (any, error) {
	if len(operations) == 0 {
		return nil, nil
	}

	endpoint := c.endpoint + "/configure"
	jsonOperations, err := json.Marshal(operations)
	if err != nil {
		return nil, fmt.Errorf("fail json marshal ops: %w", err)
	}

	payload := url.Values{
		"key":  []string{c.apiKey},
		"data": []string{string(jsonOperations)},
	}

	payloadEncoded := payload.Encode()
	tools.Info(ctx, "Creating configure request for endpoint", map[string]interface{}{"endpoint": endpoint, "operationCount": len(operations)})

	backOffDelay := 500 * time.Millisecond
	gatewayBackOffDelay := gatewayRetryInitialDelay
	const (
		lockBackOffMultiplier = 1.5
		lockBackOffMax        = 10 * time.Second
	)

	for attempt := 0; ; attempt++ {
		resp, err := c.doRequestWithRetry(ctx, func() (*http.Request, error) {
			req, reqErr := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(payloadEncoded))
			if reqErr != nil {
				return nil, fmt.Errorf("failed to create http request object: %w", reqErr)
			}

			req.Header.Set("User-Agent", c.userAgent)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return req, nil
		})
		if err != nil {
			return nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("failed to read http response: %w", readErr)
		}

		if resp.StatusCode == http.StatusBadGateway {
			tools.Warn(ctx, "VyOS API returned 502 Bad Gateway, retrying configure request", map[string]interface{}{"attempt": attempt, "backOff": gatewayBackOffDelay})
			if err := waitForLock(ctx, gatewayBackOffDelay); err != nil {
				return nil, fmt.Errorf("commit aborted while waiting for gateway recovery: %w", err)
			}
			gatewayBackOffDelay = increaseGatewayBackOff(gatewayBackOffDelay)
			continue
		}

		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("http error [%s]: %s", resp.Status, string(body))
		}

		var ret map[string]interface{}
		if err := json.Unmarshal(body, &ret); err != nil {
			return nil, fmt.Errorf("failed to unmarshal http response body: '%s' as json: %w", body, err)
		}

		if ret["success"] == true {
			return ret["data"], nil
		}

		apiErr := fmt.Errorf("API ERROR [%s]: %v", resp.Status, ret["error"])
		if isCommitLockError(ret["error"]) {
			delay := backOffDelay
			if delay > lockBackOffMax {
				delay = lockBackOffMax
			}

			tools.Warn(ctx, "VyOS configuration locked, retrying commit", map[string]interface{}{"attempt": attempt, "backOff": delay})
			if err := waitForLock(ctx, delay); err != nil {
				return nil, fmt.Errorf("commit aborted while waiting for lock to clear: %w", err)
			}

			if backOffDelay < lockBackOffMax {
				backOffDelay = time.Duration(float64(backOffDelay) * lockBackOffMultiplier)
				if backOffDelay > lockBackOffMax {
					backOffDelay = lockBackOffMax
				}
			}

			continue
		}

		tools.Warn(ctx, "VyOS configure request failed", map[string]interface{}{
			"status":     resp.Status,
			"error":      ret["error"],
			"operations": operations,
		})
		return nil, apiErr
	}
}

func (c *Client) resourceKey(resourcePath []string) string {
	if len(resourcePath) == 0 {
		return "__global__"
	}
	return strings.Join(resourcePath, " ")
}

func (c *Client) bindingKey(resourcePath []string) string {
	joined := c.resourceKey(resourcePath)

	c.bindingMu.RLock()
	defer c.bindingMu.RUnlock()
	for _, override := range c.bindingOverrides {
		if strings.HasPrefix(joined, override.prefix) {
			return override.bindAs
		}
	}

	return joined
}

func waitForLock(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func increaseGatewayBackOff(current time.Duration) time.Duration {
	if current >= gatewayRetryMaxDelay {
		return gatewayRetryMaxDelay
	}

	next := time.Duration(float64(current) * gatewayRetryMultiplier)
	if next > gatewayRetryMaxDelay {
		return gatewayRetryMaxDelay
	}
	if next <= 0 {
		return gatewayRetryInitialDelay
	}
	return next
}

func (c *Client) doRequestWithRetry(ctx context.Context, requestFactory func() (*http.Request, error)) (*http.Response, error) {
	if c.requestRetryAttempts <= 0 {
		return c.singleRequest(ctx, requestFactory)
	}

	delay := httpRetryInitialDelay
	for attempt := 0; ; attempt++ {
		resp, err := c.singleRequest(ctx, requestFactory)
		if err == nil {
			return resp, nil
		}

		if attempt >= c.requestRetryAttempts {
			return nil, err
		}

		tools.Warn(ctx, "HTTP request failed, retrying", map[string]interface{}{"attempt": attempt + 1, "maxAttempts": c.requestRetryAttempts + 1, "error": err, "backOff": delay})
		if waitErr := waitForLock(ctx, delay); waitErr != nil {
			return nil, fmt.Errorf("request aborted while waiting to retry: %w", waitErr)
		}
		delay = increaseHTTPRequestBackOff(delay)
	}
}

func (c *Client) singleRequest(ctx context.Context, requestFactory func() (*http.Request, error)) (*http.Response, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
	req, err := requestFactory()
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to complete http request: %w", err)
	}
	return resp, nil
}

func increaseHTTPRequestBackOff(current time.Duration) time.Duration {
	if current >= httpRetryMaxDelay {
		return httpRetryMaxDelay
	}

	next := time.Duration(float64(current) * httpRetryMultiplier)
	if next > httpRetryMaxDelay {
		return httpRetryMaxDelay
	}
	if next <= 0 {
		return httpRetryInitialDelay
	}
	return next
}

func isCommitLockError(errVal any) bool {
	msg, ok := errVal.(string)
	if !ok {
		msg = fmt.Sprint(errVal)
	}

	return strings.Contains(strings.ToLower(msg), "configuration system temporarily locked")
}

// Has checks the provided path for a configuration and returns
// true if found, false otherwise.
// Also returns true for empty config blocks by
// using the `exists` API operation.
func (c *Client) Has(ctx context.Context, path []string) (bool, error) {
	endpoint := c.endpoint + "/retrieve"
	operation, err := json.Marshal(
		map[string]interface{}{
			"op":   "exists",
			"path": path,
		},
	)
	if err != nil {
		return false, &MarshalError{message: "read operation", marshalErr: err}
	}

	if deadline, ok := ctx.Deadline(); ok {
		dur := time.Until(deadline)
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, dur/3)
		defer cancel()
	}

	payload := url.Values{
		"key":  []string{c.apiKey},
		"data": []string{string(operation)},
	}

	tools.Debug(ctx, "Creating 'exists' request for endpoint", map[string]interface{}{"endpoint": endpoint, "payload": payload})

	payloadEnc := payload.Encode()
	tools.Trace(ctx, "Request payload encoded", map[string]interface{}{"payload": payloadEnc})
	gatewayBackOffDelay := gatewayRetryInitialDelay

	for attempt := 0; ; attempt++ {
		resp, err := c.doRequestWithRetry(ctx, func() (*http.Request, error) {
			req, reqErr := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(payloadEnc))
			tools.Trace(ctx, "Request created", map[string]interface{}{"error": reqErr})
			if reqErr != nil {
				return nil, reqErr
			}

			req.Header.Set("User-Agent", c.userAgent)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			tools.Trace(ctx, "Request headers set")
			return req, nil
		})
		tools.Trace(ctx, "Request complete", map[string]interface{}{"error": err})
		if err != nil {
			return false, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			tools.Trace(ctx, "failed to read http response", map[string]interface{}{"error": err})
			return false, fmt.Errorf("failed to read http response: %w", err)
		}

		if resp.StatusCode == http.StatusBadGateway {
			tools.Warn(ctx, "VyOS API returned 502 Bad Gateway, retrying exists request", map[string]interface{}{"attempt": attempt, "backOff": gatewayBackOffDelay})
			if err := waitForLock(ctx, gatewayBackOffDelay); err != nil {
				return false, fmt.Errorf("request aborted while waiting for gateway recovery: %w", err)
			}
			gatewayBackOffDelay = increaseGatewayBackOff(gatewayBackOffDelay)
			continue
		}

		var ret map[string]interface{}

		err = json.Unmarshal(body, &ret)
		if err != nil {
			tools.Trace(ctx, "failed to unmarshal http response body", map[string]interface{}{"error": err, "body": body})
			return false, fmt.Errorf("failed to unmarshal http response body: '%s' as json: %w", body, err)
		}

		if ret["success"] == true {

			if retB, ok := ret["data"].(bool); ok {
				tools.Trace(ctx, "resource check complete", map[string]interface{}{"result": retB})
				return retB, nil
			}
			tools.Trace(ctx, "[api error]: could not convert returned 'data' field to bool", map[string]interface{}{"ret": ret})
			return false, fmt.Errorf("[api error]: could not convert returned 'data' field to bool: %v", ret)
		}

		if errmsg, ok := ret["error"]; ok {
			if errmsg, ok := errmsg.(string); ok {
				tools.Trace(ctx, "[api error]", map[string]interface{}{"errmsg": errmsg})
				return false, clienterrors.NewNotFoundError("[api error]: %s", errmsg)
			}
		}

		tools.Trace(ctx, "[api error]", map[string]interface{}{"ret": ret})
		return false, clienterrors.NewNotFoundError("[api error]: %v", ret)
	}
}

// Get returns the config found under path if it exists
//
// Returns:
//
//	error: if the resource was not found a clienterror.NotFoundError is returned, otherwise a generic error
func (c *Client) Get(ctx context.Context, path []string) (any, error) {
	endpoint := c.endpoint + "/retrieve"
	operation, err := json.Marshal(
		map[string]interface{}{
			"op":   "showConfig",
			"path": path,
		},
	)
	if err != nil {
		return nil, &MarshalError{message: "showConfig operation", marshalErr: err}
	}

	payload := url.Values{
		"key":  []string{c.apiKey},
		"data": []string{string(operation)},
	}

	tools.Info(ctx, "Creating showConfig request for endpoint", map[string]interface{}{"endpoint": endpoint, "payload": payload})

	payloadEnc := payload.Encode()
	tools.Debug(ctx, "Request payload encoded", map[string]interface{}{"payload": payloadEnc})
	gatewayBackOffDelay := gatewayRetryInitialDelay

	for attempt := 0; ; attempt++ {
		resp, err := c.doRequestWithRetry(ctx, func() (*http.Request, error) {
			req, reqErr := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(payloadEnc))
			if reqErr != nil {
				return nil, fmt.Errorf("failed to create http request object: %w", reqErr)
			}

			req.Header.Set("User-Agent", c.userAgent)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			return req, nil
		})
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read http response: %w", err)
		}

		if resp.StatusCode == http.StatusBadGateway {
			tools.Warn(ctx, "VyOS API returned 502 Bad Gateway, retrying showConfig request", map[string]interface{}{"attempt": attempt, "backOff": gatewayBackOffDelay})
			if err := waitForLock(ctx, gatewayBackOffDelay); err != nil {
				return nil, fmt.Errorf("request aborted while waiting for gateway recovery: %w", err)
			}
			gatewayBackOffDelay = increaseGatewayBackOff(gatewayBackOffDelay)
			continue
		}

		var ret map[string]interface{}

		err = json.Unmarshal(body, &ret)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal http response body: '%s' as json: %w", body, err)
		}

		if ret["success"] == true {

			return ret["data"], nil
		}

		if errmsg, ok := ret["error"]; ok {
			if errmsg, ok := errmsg.(string); ok && errmsg == "Configuration under specified path is empty\n" {
				return nil, clienterrors.NewNotFoundError(
					"[%s]: %s",
					strings.Join(path, " "),
					strings.TrimSuffix(errmsg, "\n"),
				)
			}

			return nil, fmt.Errorf("[api error]: %s", errmsg)
		}

		return nil, fmt.Errorf("[api error]: %v", ret)
	}
}
