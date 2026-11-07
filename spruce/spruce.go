package spruce

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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
	DetectSquash     bool
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
	base := gs.firstValidBase()
	if base == "" {
		return false
	}

	return gs.isMerged(branchName, base)
}

func (gs *GitSpruce) firstValidBase() string {
	var toCheck []string
	if gs.MergeBase == "master" || gs.MergeBase == "main" {
		toCheck = []string{"master", "main"}
	} else {
		toCheck = []string{gs.MergeBase}
	}

	for _, base := range toCheck {
		if gs.isValidReference(base) {
			return base
		}
	}

	return ""
}

func (gs *GitSpruce) isValidReference(refName string) bool {
	// equivalent to git rev-parse --quiet --verify %s
	// ResolveRevision accepts shorthand names like "main" as well as full ref names.
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

var (
	// hunk headers carry line numbers that shift when the same change lands
	// on a different base, so they are stripped before hashing, as git
	// patch-id does
	hunkHeaderRe = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)
	indexLineRe  = regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+.*$`)
)

const maxSquashScanCommits = 1000

// BranchIsSquashMerged reports whether the branch's changes landed on the
// merge base as a squash merge: the branch tip is not an ancestor of the
// base, but the diff from the merge base to the branch tip matches the diff
// of some commit on the base.
func (gs *GitSpruce) BranchIsSquashMerged(branchName string) bool {
	base := gs.firstValidBase()
	if base == "" {
		return false
	}

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

	mergeBases, err := branchCommit.MergeBase(baseCommit)
	if err != nil || len(mergeBases) == 0 {
		return false
	}

	branchPatch, ok := commitRangePatchHash(mergeBases[0], branchCommit)
	if !ok {
		return false
	}

	iter, err := gs.Repo.Log(&git.LogOptions{From: *baseHash})
	if err != nil {
		return false
	}
	defer iter.Close()

	for i := 0; i < maxSquashScanCommits; i++ {
		commit, err := iter.Next()
		if err != nil {
			return false
		}

		if commit.Hash == mergeBases[0].Hash {
			return false
		}

		// only plain commits have a single parent diff; merge commits on
		// the base cannot be a squash of the branch
		if len(commit.ParentHashes) != 1 {
			continue
		}

		parent, err := gs.Repo.CommitObject(commit.ParentHashes[0])
		if err != nil {
			continue
		}

		if patch, ok := commitRangePatchHash(parent, commit); ok && patch == branchPatch {
			return true
		}
	}

	return false
}

// commitRangePatchHash hashes the normalised diff a commit range introduces,
// comparable across branches. ok is false when the range changes nothing.
func commitRangePatchHash(from, to *object.Commit) (hash string, ok bool) {
	fromTree, err := from.Tree()
	if err != nil {
		return "", false
	}

	toTree, err := to.Tree()
	if err != nil {
		return "", false
	}

	changes, err := object.DiffTree(fromTree, toTree)
	if err != nil || len(changes) == 0 {
		return "", false
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", false
	}

	normalized := indexLineRe.ReplaceAllString(hunkHeaderRe.ReplaceAllString(patch.String(), "@@ @@"), "")
	sum := sha1.Sum([]byte(normalized))

	return hex.EncodeToString(sum[:]), true
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
	if !gs.Force && !gs.isMerged(branchName, gs.MergeBase) && !(gs.DetectSquash && gs.BranchIsSquashMerged(branchName)) {
		return false, fmt.Errorf("branch %s is not merged; use Force to delete anyway", branchName)
	}

	return true, gs.Repo.Storer.RemoveReference(ref)
}
