package spruce

import (
	"crypto/sha1" //nolint:gosec // patch-id matching mirrors git patch-id; not a security hash
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
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

	// caches: go-git re-reads ref and object files from disk on every
	// lookup, which dominates when checking many branches
	base        *object.Commit
	commits     map[plumbing.Hash]*object.Commit
	basePatches map[string]struct{}
	head        plumbing.ReferenceName
}

func (gs *GitSpruce) Fetch(prune bool) error {
	// equivalent to git fetch
	return gs.Repo.Fetch(&git.FetchOptions{
		RemoteName: gs.Origin,
		Prune:      prune,
		Auth:       gs.fetchAuth(),
	})
}

// defaultSSHKeyFiles are the identity files the ssh client tries when no
// agent is available, in the same order.
var defaultSSHKeyFiles = []string{"id_rsa", "id_ecdsa", "id_ed25519", "id_dsa"}

// go-git does not pick up the system SSH agent on its own, so SSH remotes
// need explicit auth or the fetch is rejected as unauthenticated. The agent
// is preferred, but without SSH_AUTH_SOCK (cron, systemd, CI) it cannot be
// reached, so fall back to the default identity files, as git does.
func (gs *GitSpruce) fetchAuth() transport.AuthMethod {
	remote, err := gs.Repo.Remote(gs.Origin)
	if err != nil {
		return nil
	}

	url := remote.Config().URLs[0]
	if !strings.HasPrefix(url, "git@") && !strings.HasPrefix(url, "ssh://") {
		return nil
	}

	if auth, err := gitssh.NewSSHAgentAuth("git"); err == nil {
		return auth
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	for _, keyFile := range defaultSSHKeyFiles {
		keyPath := filepath.Join(home, ".ssh", keyFile)
		if _, statErr := os.Stat(keyPath); statErr != nil {
			continue
		}
		if auth, err := gitssh.NewPublicKeysFromFile("git", keyPath, ""); err == nil {
			return auth
		}
	}

	return nil
}

func (gs *GitSpruce) LoadBranches() ([]GitBranch, error) {
	refs, err := gs.Repo.References()
	if err != nil {
		return nil, err
	}

	// one pass over all refs: collect branch refs and remote refs, then
	// mark gone branches from the in-memory set
	remoteRefs := map[string]struct{}{}
	var locals []string

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()

		if name.IsRemote() {
			remoteRefs[name.String()] = struct{}{}
			return nil
		}

		if name.IsBranch() {
			branchName := name.Short()
			if !gs.isBranchIgnored(branchName) {
				locals = append(locals, branchName)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	branchList := make([]GitBranch, 0, len(locals))

	cfg, err := gs.Repo.Config()
	if err != nil {
		return nil, err
	}

	for _, branchName := range locals {
		// git branch -vv flags a branch [gone] only when its configured
		// upstream's remote-tracking ref no longer exists; a branch with
		// no upstream was never pushed and must never be flagged, or
		// --gone would delete unpushed work
		upstream := cfg.Branches[branchName]
		upstreamRef := ""
		if upstream != nil && upstream.Remote != "" && upstream.Merge != "" {
			upstreamRef = plumbing.NewRemoteReferenceName(upstream.Remote, upstream.Merge.Short()).String()
		}

		_, refExists := remoteRefs[upstreamRef]

		branchList = append(branchList, GitBranch{
			Name:   branchName,
			IsGone: upstreamRef != "" && !refExists,
		})
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

// baseCommit resolves and caches the merge base commit.
func (gs *GitSpruce) baseCommit() *object.Commit {
	if gs.base != nil {
		return gs.base
	}

	name := gs.firstValidBase()
	if name == "" {
		return nil
	}

	hash, err := gs.Repo.ResolveRevision(plumbing.Revision(name))
	if err != nil {
		return nil
	}

	commit, err := gs.commitObject(*hash)
	if err != nil {
		return nil
	}

	gs.base = commit

	return commit
}

// branchCommit resolves a branch name straight to its ref hash, falling
// back to revision parsing for non-branch names, and caches the commit.
func (gs *GitSpruce) branchCommit(branchName string) (*object.Commit, bool) {
	ref, err := gs.Repo.Reference(plumbing.NewBranchReferenceName(branchName), false)
	if err != nil {
		hash, rerr := gs.Repo.ResolveRevision(plumbing.Revision(branchName))
		if rerr != nil {
			return nil, false
		}
		ref = plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), *hash)
	}

	commit, err := gs.commitObject(ref.Hash())
	if err != nil {
		return nil, false
	}

	return commit, true
}

func (gs *GitSpruce) commitObject(hash plumbing.Hash) (*object.Commit, error) {
	if gs.commits == nil {
		gs.commits = map[plumbing.Hash]*object.Commit{}
	}

	if commit, ok := gs.commits[hash]; ok {
		return commit, nil
	}

	commit, err := gs.Repo.CommitObject(hash)
	if err != nil {
		return nil, err
	}

	gs.commits[hash] = commit

	return commit, nil
}

// BranchTipHash resolves a branch to the hash of its tip commit, for
// reporting before a deletion removes the ref.
func (gs *GitSpruce) BranchTipHash(branchName string) (plumbing.Hash, bool) {
	commit, ok := gs.branchCommit(branchName)
	if !ok {
		return plumbing.ZeroHash, false
	}

	return commit.Hash, true
}

func (gs *GitSpruce) BranchIsMerged(branchName string) bool {
	base := gs.baseCommit()
	if base == nil {
		return false
	}

	branchCommit, ok := gs.branchCommit(branchName)
	if !ok {
		return false
	}

	return gs.isMerged(branchCommit, base)
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

func (gs *GitSpruce) isMerged(branchCommit, baseCommit *object.Commit) bool {
	// equivalent to git merge-base --is-ancestor
	// https://github.com/go-git/go-git/blob/master/_examples/merge_base/main.go

	isAncestor, err := branchCommit.IsAncestor(baseCommit)
	if err != nil {
		return false
	}

	return isAncestor
}

var (
	// hunk headers carry line numbers that shift when the same change
	// lands on a different base, so they are stripped before hashing, as
	// git patch-id does.
	hunkHeaderRe = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)
	indexLineRe  = regexp.MustCompile(`(?m)^index [0-9a-f]+\.\.[0-9a-f]+.*$`)
)

const maxSquashScanCommits = 1000

// BranchIsSquashMerged reports whether the branch's changes landed on the
// merge base as a squash merge: the branch tip is not an ancestor of the
// base, but the diff from the merge base to the branch tip matches the diff
// of some commit on the base.
func (gs *GitSpruce) BranchIsSquashMerged(branchName string) bool {
	base := gs.baseCommit()
	if base == nil {
		return false
	}

	branchCommit, ok := gs.branchCommit(branchName)
	if !ok {
		return false
	}

	mergeBases, err := branchCommit.MergeBase(base)
	if err != nil || len(mergeBases) == 0 {
		return false
	}

	branchPatch, ok := commitRangePatchHash(mergeBases[0], branchCommit)
	if !ok {
		return false
	}

	_, match := gs.basePatchSet()[branchPatch]

	return match
}

// basePatchSet lazily hashes the diff of every plain commit on the base,
// once, so N squash checks against the base cost one walk, not N.
func (gs *GitSpruce) basePatchSet() map[string]struct{} {
	if gs.basePatches != nil {
		return gs.basePatches
	}

	set := map[string]struct{}{}
	gs.basePatches = set

	base := gs.baseCommit()
	if base == nil {
		return set
	}

	iter, err := gs.Repo.Log(&git.LogOptions{From: base.Hash})
	if err != nil {
		return set
	}
	defer iter.Close()

	for range maxSquashScanCommits {
		commit, nextErr := iter.Next()
		if nextErr != nil {
			return set
		}

		// only plain commits have a single parent diff; merge commits on
		// the base cannot be a squash of the branch
		if len(commit.ParentHashes) != 1 {
			continue
		}

		parent, parentErr := gs.commitObject(commit.ParentHashes[0])
		if parentErr != nil {
			continue
		}

		if hash, ok := commitRangePatchHash(parent, commit); ok {
			set[hash] = struct{}{}
		}
	}

	return set
}

// commitRangePatchHash hashes the normalised diff a commit range
// introduces, comparable across branches. The second return value is
// false when the range changes nothing.
func commitRangePatchHash(from, to *object.Commit) (string, bool) {
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
	//nolint:gosec // identical to git patch-id, which also hashes with SHA-1
	sum := sha1.Sum([]byte(normalized))

	return hex.EncodeToString(sum[:]), true
}

func (gs *GitSpruce) headName() plumbing.ReferenceName {
	if gs.head != "" {
		return gs.head
	}

	head, err := gs.Repo.Head()
	if err != nil {
		return ""
	}

	gs.head = head.Name()

	return gs.head
}

func (gs *GitSpruce) DeleteBranch(branchName string) (bool, error) {
	return gs.deleteBranch(branchName, false)
}

// DeleteGoneBranch removes a branch flagged [gone]: its remote-tracking
// ref no longer exists, so the upstream has already discarded it and the
// merged check does not apply.
func (gs *GitSpruce) DeleteGoneBranch(branchName string) (bool, error) {
	return gs.deleteBranch(branchName, true)
}

func (gs *GitSpruce) deleteBranch(branchName string, gone bool) (bool, error) {
	ref := plumbing.NewBranchReferenceName(branchName)

	// as with git branch -d, refuse to delete the checked-out branch
	if gs.headName() == ref {
		return false, fmt.Errorf("cannot delete branch %s: it is the current branch", branchName)
	}

	// without Force, behave like git branch -d and refuse unmerged branches
	if !gs.Force && !gone {
		branchCommit, ok := gs.branchCommit(branchName)
		if !ok {
			return false, fmt.Errorf("branch %s not found", branchName)
		}

		base := gs.baseCommit()
		if base == nil {
			return false, fmt.Errorf("cannot delete branch %s: merge base %s could not be resolved", branchName, gs.MergeBase)
		}

		if !gs.isMerged(branchCommit, base) && !(gs.DetectSquash && gs.BranchIsSquashMerged(branchName)) {
			return false, fmt.Errorf("branch %s is not merged; use Force to delete anyway", branchName)
		}
	}

	if err := gs.Repo.Storer.RemoveReference(ref); err != nil {
		return false, err
	}

	return true, nil
}
