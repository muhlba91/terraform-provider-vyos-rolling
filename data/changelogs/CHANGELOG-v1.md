
# CHANGELOG

<!--TOC-->

- [CHANGELOG](#changelog)
  - [Release 1.0.202507150 (2025-11-23 12-26-24 UTC)](#release-10202507150-2025-11-23-12-26-24-utc)
    - [Project changes](#project-changes)
      - [Notes](#notes)
    - [Schema changes](#schema-changes)
      - [BREAKING CHANGES](#breaking-changes)
        - [Resources](#resources)
      - [Notes](#notes-1)
        - [Resources](#resources-1)
      - [Features](#features)
        - [Resources](#resources-2)
  - [Previous changelogs](#previous-changelogs)

<!--TOC-->


## Release 1.0.202507150 (2025-11-23 12-26-24 UTC)
### Project changes
#### Notes
* add testing instructions for SSH service changes and update service documentation
* update development workflow

### Schema changes
#### BREAKING CHANGES

##### Resources
* Modified Resource `vyos_load_balancing_haproxy_service`
	* Attribute `backend`changed `description`
	* New attribute `timeout`
	* **Removed attribute** `listen_address`

* **Removed Resource** `vyos_service_ids_ddos_protection_threshold_general`

* **Removed Resource** `vyos_service_ids_ddos_protection_threshold_udp`

* **Removed Resource** `vyos_service_ids_ddos_protection_threshold_icmp`

* **Removed Resource** `vyos_service_ssh_trusted_user_ca_key`

* **Removed Resource** `vyos_service_ids_ddos_protection_threshold_tcp`

* Modified Resource `vyos_protocols_mpls_ldp`
	* **Removed attribute** `interface`

* Modified Resource `vyos_service_lldp_interface`
	* New attribute `mode`
	* **Removed attribute** `disable`

* Modified Resource `vyos_service_ipoe_server_authentication_interface_mac`
	* New attribute `ip_address`
	* **Removed attribute** `static_ip`

* **Removed Resource** `vyos_service_ids_ddos_protection`

* Modified Resource `vyos_service_dhcp_server`
	* **Removed attribute** `dynamic_dns_update`

* Modified Resource `vyos_firewall_global_options_apply_to_bridged_traffic`
	* **Removed attribute** `invalid_connections`

* **Removed Resource** `vyos_service_ids_ddos_protection_sflow`





#### Notes

##### Resources
* Modified Resource `vyos_load_balancing_haproxy_service_logging_facility`
	* Modified attribute `identifier`
		* Attribute `facility`changed `description`
	* Attribute `level`changed `description`

* Modified Resource `vyos_vpn_l2tp_remote_access_authentication_radius_dynamic_author`
	* Attribute `port`changed `description`

* Modified Resource `vyos_service_ipoe_server_authentication_radius_dynamic_author`
	* Attribute `port`changed `description`

* Modified Resource `vyos_interfaces_wireguard_peer`
	* Attribute `preshared_key`changed `description`
	* Attribute `public_key`changed `description`

* Modified Resource `vyos_vpn_sstp_authentication_radius_dynamic_author`
	* Attribute `port`changed `description`

* Modified Resource `vyos_policy_community_list_rule`
	* Attribute `regex`changed `description`

* Modified Resource `vyos_service_pppoe_server_authentication_radius_dynamic_author`
	* Attribute `port`changed `description`

* Modified Resource `vyos_load_balancing_haproxy_global_parameters_logging_facility`
	* Modified attribute `identifier`
		* Attribute `facility`changed `description`
	* Attribute `level`changed `description`

* Modified Resource `vyos_load_balancing_haproxy_backend_logging_facility`
	* Modified attribute `identifier`
		* Attribute `facility`changed `description`
	* Attribute `level`changed `description`

* Modified Resource `vyos_vpn_pptp_remote_access_authentication_radius_dynamic_author`
	* Attribute `port`changed `description`

* Modified Resource `vyos_load_balancing_haproxy_service_rule`
	* Modified attribute `set`
		* Attribute `redirect_location`changed `description`

* Modified Resource `vyos_system_conntrack`
	* Attribute `hash_size`changed `description`

* Modified Resource `vyos_system_syslog_remote`
	* Modified attribute `format`
		* Attribute `include_timezone`changed `description`
		* Attribute `octet_counted`changed `description`





#### Features

##### Resources
* Modified Resource `vyos_system_option_kernel`
	* New attribute `disable_hpet`
	* New attribute `disable_mce`
	* New attribute `quiet`
	* New attribute `disable_softlockup`

* Modified Resource `vyos_service_dhcpv6_server_shared_network_name_subnet_static_mapping`
	* Attribute `option`New attribute `option.capwap_controller`

* Modified Resource `vyos_protocols_babel_redistribute_ipv4`
	* New attribute `nhrp`

* Modified Resource `vyos_container_name`
	* Attribute `capability`changed `description`
	* New attribute `privileged`
	* New attribute `log_driver`

* Modified Resource `vyos_interfaces_ethernet_vif_s_vif_c`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_firewall_ipv6_output_filter_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_vpn_pptp_remote_access`
	* New attribute `thread_count`

* Modified Resource `vyos_policy_route_map_rule`
	* Attribute `match`New attribute `match.source_vrf`
	* Modified attribute `set`
		* Modified attribute `set.community.community`
			* Attribute `add`changed `description`
			* Attribute `replace`changed `description`

* Modified Resource `vyos_service_dhcp_server_shared_network_name`
	* Attribute `option`New attribute `option.capwap_controller`
	* Modified attribute `subnet`
		* Attribute `option`New attribute `subnet.option.option.capwap_controller`
		* Modified attribute `subnet.range.range`
			* Attribute `option`New attribute `subnet.range.range.option.option.capwap_controller`
		* New attribute `subnet.ping_check.ping_check`
		* New attribute `subnet.dynamic_dns_update.dynamic_dns_update`
	* New attribute `ping_check`
	* New attribute `dynamic_dns_update`

* Modified Resource `vyos_service_dhcpv6_server_shared_network_name_subnet_range`
	* Attribute `option`New attribute `option.capwap_controller`

* Modified Resource `vyos_interfaces_bridge_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_protocols_rpki_cache`
	* New attribute `source_address`

* Modified Resource `vyos_interfaces_l2tpv3`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_wireless_vif_s_vif_c`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_virtual_ethernet_vif_s_vif_c`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_wireless`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_pseudo_ethernet_vif_s`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_system_option`
	* New attribute `reboot_on_upgrade_failure`

* Modified Resource `vyos_firewall_ipv4_forward_filter_rule`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`

* Modified Resource `vyos_firewall_ipv4_name_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_interfaces_bonding_vif_s`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_ethernet_vif_s`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_service_dhcpv6_server_shared_network_name`
	* Attribute `option`New attribute `option.capwap_controller`

* Modified Resource `vyos_interfaces_wireless_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_service_pppoe_server`
	* New attribute `thread_count`

* Modified Resource `vyos_interfaces_openvpn`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_macsec`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_wwan`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_firewall_ipv6_input_filter_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_firewall_ipv6_name_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_policy_route_rule`
	* Attribute `destination`New attribute `destination.geoip`
	* Attribute `source`New attribute `source.geoip`

* Modified Resource `vyos_firewall_ipv6_forward_filter_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_interfaces_virtual_ethernet_vif_s`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_firewall_ipv4_output_filter_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_protocols_bgp_parameters`
	* New attribute `no_ipv6_auto_ra`

* Modified Resource `vyos_interfaces_pseudo_ethernet_vif_s_vif_c`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_wireless_vif_s`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_vpn_sstp`
	* New attribute `thread_count`

* Modified Resource `vyos_vpn_ipsec_site_to_site_peer`
	* Attribute `vti`New attribute `vti.traffic_selector`

* Modified Resource `vyos_interfaces_virtual_ethernet`
	* New attribute `mtu`

* Modified Resource `vyos_service_ssh`
	* New attribute `trusted_user_ca`

* Modified Resource `vyos_interfaces_bridge`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_firewall_ipv4_input_filter_rule`
	* Modified attribute `source`
		* Attribute `group`New attribute `source.group.group.remote_group`
	* Modified attribute `destination`
		* Attribute `group`New attribute `destination.group.group.remote_group`

* Modified Resource `vyos_service_dhcp_server_shared_network_name_subnet_static_mapping`
	* Attribute `option`New attribute `option.capwap_controller`

* Modified Resource `vyos_interfaces_bonding_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_system_login_user`
	* Attribute `authentication`New attribute `authentication.principal`

* Modified Resource `vyos_interfaces_pseudo_ethernet`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_qos_policy_cake`
	* New attribute `ack_filter`
	* New attribute `no_split_gso`

* Modified Resource `vyos_interfaces_bonding_vif_s_vif_c`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_service_ipoe_server`
	* New attribute `thread_count`

* Modified Resource `vyos_service_router_advert_interface`
	* New attribute `auto_ignore`

* Modified Resource `vyos_vpn_l2tp_remote_access`
	* New attribute `thread_count`

* Modified Resource `vyos_interfaces_ethernet_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_vrf_name`
	* Modified attribute `protocols`
		* Modified attribute `protocols.bgp.bgp`
			* Modified attribute `protocols.bgp.bgp.address_family.address_family`
				* Modified attribute `protocols.bgp.bgp.address_family.address_family.ipv6_unicast.ipv6_unicast`
					* Attribute `redistribute`New attribute `protocols.bgp.bgp.address_family.address_family.ipv6_unicast.ipv6_unicast.redistribute.redistribute.nhrp`
				* Modified attribute `protocols.bgp.bgp.address_family.address_family.ipv4_unicast.ipv4_unicast`
					* Attribute `redistribute`New attribute `protocols.bgp.bgp.address_family.address_family.ipv4_unicast.ipv4_unicast.redistribute.redistribute.nhrp`
					* Attribute `route_map`New attribute `protocols.bgp.bgp.address_family.address_family.ipv4_unicast.ipv4_unicast.route_map.route_map.vrf`
			* Attribute `parameters`New attribute `protocols.bgp.bgp.parameters.parameters.no_ipv6_auto_ra`
		* Modified attribute `protocols.isis.isis`
			* Modified attribute `protocols.isis.isis.redistribute.redistribute`
				* Attribute `ipv4`New attribute `protocols.isis.isis.redistribute.redistribute.ipv4.ipv4.nhrp`
		* Modified attribute `protocols.ospf.ospf`
			* Attribute `redistribute`New attribute `protocols.ospf.ospf.redistribute.redistribute.nhrp`
		* New attribute `protocols.rpki.rpki`

* Modified Resource `vyos_interfaces_bonding`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_protocols_babel_redistribute_ipv6`
	* New attribute `nhrp`

* Modified Resource `vyos_interfaces_ethernet`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_pppoe`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_system_syslog_marker`
	* New attribute `disable`

* Modified Resource `vyos_container_registry`
	* New attribute `insecure`
	* New attribute `mirror`

* Modified Resource `vyos_interfaces_bridge_member_interface`
	* New attribute `bpdu_guard`
	* New attribute `root_guard`

* Modified Resource `vyos_service_dhcpv6_server_shared_network_name_subnet`
	* Attribute `option`New attribute `option.capwap_controller`

* Modified Resource `vyos_interfaces_virtual_ethernet_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_interfaces_geneve`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`
	* New attribute `port`

* Modified Resource `vyos_interfaces_pseudo_ethernet_vif`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* Modified Resource `vyos_policy_route6_rule`
	* Attribute `destination`New attribute `destination.geoip`
	* Attribute `source`New attribute `source.geoip`

* Modified Resource `vyos_interfaces_vxlan`
	* Modified attribute `ipv6`
		* Attribute `address`New attribute `ipv6.address.address.interface_identifier`

* New Resource `vyos_firewall_global_options_state_policy_offload`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update_tsig_key`

* New Resource `vyos_protocols_bgp_address_family_ipv6_unicast_redistribute_nhrp`

* New Resource `vyos_protocols_mpls_ldp_interface`

* New Resource `vyos_system_option_kernel_memory`

* New Resource `vyos_protocols_rip_redistribute_nhrp`

* New Resource `vyos_firewall_global_options_apply_to_bridged_traffic_accept_invalid`

* New Resource `vyos_system_option_kernel_memory_hugepage_size`

* New Resource `vyos_firewall_group_remote_group`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update_reverse_domain`

* New Resource `vyos_protocols_ospf_redistribute_nhrp`

* New Resource `vyos_container_name_tmpfs`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update`

* New Resource `vyos_system_ip_import_table`

* New Resource `vyos_system_option_kernel_cpu`

* New Resource `vyos_protocols_bgp_address_family_ipv4_unicast_redistribute_nhrp`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update_forward_domain_dns_server`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update_reverse_domain_dns_server`

* New Resource `vyos_protocols_isis_redistribute_ipv4_nhrp_level_2`

* New Resource `vyos_load_balancing_haproxy_timeout`

* New Resource `vyos_protocols_isis_redistribute_ipv4_nhrp_level_1`

* New Resource `vyos_service_dhcp_server_dynamic_dns_update_forward_domain`

* New Resource `vyos_load_balancing_haproxy_service_listen_address`

* New Resource `vyos_protocols_bgp_address_family_ipv4_unicast_route_map_vrf`

* New Resource `vyos_vrf_name_protocols_rpki_cache`








## Previous changelogs
For previous version see [changelog for v20](CHANGELOG-v20.md) or older archives [directory](data/changelogs/)
