package spruce

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type GitBranch struct {
	Name   string
	IsGone bool
}

type GitSpruce struct {
	MergeBase        string
	BranchesToIgnore []string
	Origin           string
	Force            bool
	Repo             *git.Repository
	RepoPath         string
}

func (gs *GitSpruce) Fetch(prune bool) error {
	// equivalent to git fetch
	return gs.Repo.Fetch(&git.FetchOptions{
		RemoteName: gs.Origin,
		Prune:      prune,
	})
}

func (gs *GitSpruce) LoadBranches() ([]GitBranch, error) {
	branches, err := gs.Repo.Branches()

	if err != nil {
		return nil, err
	}

	remoteRefs, err := gs.remoteRefNames()
	if err != nil {
		return nil, err
	}

	branchList := []GitBranch{}

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()

		if gs.isBranchIgnored(branchName) {
			return nil
		}

		refName := plumbing.NewRemoteReferenceName(gs.Origin, branchName)
		_, refExists := remoteRefs[refName.String()]

		branch := GitBranch{
			Name:   branchName,
			IsGone: !refExists,
		}

		branchList = append(branchList, branch)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return branchList, nil
}

func (gs *GitSpruce) isBranchIgnored(branchName string) bool {
	for _, ignoreBranch := range gs.BranchesToIgnore {
		if branchName == ignoreBranch {
			return true
		}
	}

	return false
}

// One pass over all refs, so checking N branches costs one ref walk, not N.
func (gs *GitSpruce) remoteRefNames() (map[string]struct{}, error) {
	remoteRefs := map[string]struct{}{}

	refs, err := gs.Repo.References()
	if err != nil {
		return nil, err
	}

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().IsRemote() {
			remoteRefs[ref.Name().String()] = struct{}{}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return remoteRefs, nil
}

func (gs *GitSpruce) BranchIsMerged(branchName string) bool {
	// Check if mergeBase is master or main
	var toCheck []string
	if gs.MergeBase == "master" || gs.MergeBase == "main" {
		toCheck = []string{"master", "main"}
	} else {
		toCheck = []string{gs.MergeBase}
	}

	for _, base := range toCheck {
		if gs.isValidReference(base) {
			merged := gs.isMerged(branchName, base)
			return merged
		}
	}

	return false
}

func (gs *GitSpruce) isValidReference(refName string) bool {
	// equivalent to git rev-parse --quiet --verify %s
	// ResolveRevision, not Reference, because it accepts shorthand names
	// like "main" alongside full ref names.
	_, err := gs.Repo.ResolveRevision(plumbing.Revision(refName))

	return err == nil
}

func (gs *GitSpruce) isMerged(branchName string, base string) bool {
	// equivalent to git merge-base --is-ancestor %s %s
	// https://github.com/go-git/go-git/blob/master/_examples/merge_base/main.go

	branchHash, err := gs.Repo.ResolveRevision(plumbing.Revision(branchName))
	if err != nil {
		return false
	}

	baseHash, err := gs.Repo.ResolveRevision(plumbing.Revision(base))
	if err != nil {
		return false
	}

	branchCommit, err := gs.Repo.CommitObject(*branchHash)
	if err != nil {
		return false
	}

	baseCommit, err := gs.Repo.CommitObject(*baseHash)
	if err != nil {
		return false
	}

	isAncestor, err := branchCommit.IsAncestor(baseCommit)
	if err != nil {
		return false
	}

	return isAncestor
}

func (gs *GitSpruce) DeleteBranch(branchName string) (bool, error) {
	ref := plumbing.NewBranchReferenceName(branchName)

	head, err := gs.Repo.Head()
	if err != nil {
		return false, err
	}

	// as with git branch -d, refuse to delete the checked-out branch
	if head.Name() == ref {
		return false, fmt.Errorf("cannot delete branch %s: it is the current branch", branchName)
	}

	// without Force, behave like git branch -d and refuse unmerged branches
	if !gs.Force && !gs.isMerged(branchName, gs.MergeBase) {
		return false, fmt.Errorf("branch %s is not merged; use Force to delete anyway", branchName)
	}

	return true, gs.Repo.Storer.RemoveReference(ref)
}
