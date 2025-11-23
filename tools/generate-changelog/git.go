package main

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/hashicorp/go-version"
	"github.com/leodido/go-conventionalcommits"
	"github.com/leodido/go-conventionalcommits/parser"
)

func GenerateGitChanges() (previousVersion *version.Version, commitsSinceLastVersion []conventionalcommits.Message) {
	repo, err := git.PlainOpen("../..")
	die(err)

	previousVersion, err = version.NewVersion("0.0.0")
	die(err)

	itr, err := repo.Tags()
	die(err)
	err = itr.ForEach(func(ref *plumbing.Reference) error {
		v, err := version.NewVersion(ref.Name().Short())
		if err != nil {
			// Not a valid version tag, just ignore it.
			return nil
		}
		if v.GreaterThan(previousVersion) {
			previousVersion = v
		}
		return nil
	})
	die(err)

	var releaseCommit *object.Commit
	if previousVersion.Original() != "0.0.0" {
		hash, err := repo.ResolveRevision(plumbing.Revision(previousVersion.Original()))
		if err == nil {
			c, err := repo.CommitObject(*hash)
			if err == nil {
				releaseCommit = c
			}
		}
	}

	logOptions := &git.LogOptions{
		Order: git.LogOrderCommitterTime,
	}
	if releaseCommit != nil {
		logOptions.Since = &releaseCommit.Author.When
	}

	commits, err := repo.Log(logOptions)
	die(err)

	chgs := make([]conventionalcommits.Message, 0)
	ccm := parser.NewMachine(parser.WithTypes(conventionalcommits.TypesConventional), parser.WithBestEffort())

	err = commits.ForEach(func(commit *object.Commit) error {
		if releaseCommit != nil && commit.Hash == releaseCommit.Hash {
			return nil // Skip the release commit itself
		}
		cc, _ := ccm.Parse([]byte(commit.Message))

		chgs = append(chgs, cc)
		return nil
	})
	die(err)

	return previousVersion, chgs
}