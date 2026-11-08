package spruce_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bencromwell/git-spruce/spruce"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initRepository(tempDir string, isBare bool) (*git.Repository, error) {
	if err := os.Mkdir(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	return git.PlainInit(tempDir, isBare)
}

type TestRepos struct {
	clone     *git.Repository
	origin    *git.Repository
	cloneDir  string
	originDir string
}

func createTestRepos(t *testing.T) TestRepos {
	cloneDir := filepath.Join(t.TempDir(), "test-repo-clone")
	originDir := filepath.Join(t.TempDir(), "test-repo-origin")

	clone, err := initRepository(cloneDir, false)
	require.NoError(t, err)
	origin, err := initRepository(originDir, true)
	require.NoError(t, err)

	// set the clone's remote origin to the origin repo
	_, err = clone.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{originDir},
	})
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(cloneDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0o644)
	require.NoError(t, err)

	cloneWorktree, err := clone.Worktree()
	require.NoError(t, err)
	_, err = cloneWorktree.Add("test.txt")
	require.NoError(t, err)
	_, err = cloneWorktree.Commit("Initial commit", &git.CommitOptions{})
	require.NoError(t, err)

	err = clone.Push(&git.PushOptions{})
	require.NoError(t, err)

	err = cloneWorktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/test-one"),
		Create: true,
	})
	require.NoError(t, err)

	err = cloneWorktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/test-two"),
		Create: true,
	})
	require.NoError(t, err)

	_, err = cloneWorktree.Commit("chore: commit on tmp/test-two", &git.CommitOptions{
		AllowEmptyCommits: true,
	})
	require.NoError(t, err)

	err = cloneWorktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	require.NoError(t, err)

	err = clone.Push(&git.PushOptions{})
	require.NoError(t, err)

	return TestRepos{clone, origin, cloneDir, originDir}
}

func newGitSpruce(repoPath string) (*spruce.GitSpruce, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	return &spruce.GitSpruce{
		MergeBase:        "master",
		BranchesToIgnore: []string{"main", "master", "develop"},
		Origin:           "origin",
		Force:            true,
		Repo:             repo,
		RepoPath:         repoPath,
	}, nil
}

func TestGitSpruce_Fetch(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	err = gs.Fetch(true)
	require.NoError(t, err)
}

func TestGitSpruce_LoadBranches(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	branches, err := gs.LoadBranches()
	require.NoError(t, err)
	assert.NotEmpty(t, branches)
	assert.Contains(t, branches, spruce.GitBranch{Name: "tmp/test-one", IsGone: false})
	assert.Contains(t, branches, spruce.GitBranch{Name: "tmp/test-two", IsGone: false})
	assert.Len(t, branches, 2)
}

func TestGitSpruce_BranchIsMerged(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	isMerged := gs.BranchIsMerged("tmp/test-one")
	assert.True(t, isMerged)

	isMerged = gs.BranchIsMerged("tmp/test-two")
	assert.False(t, isMerged)
}

func TestGitSpruce_BranchIsSquashMerged(t *testing.T) {
	testRepos := createTestRepos(t)

	w, err := testRepos.clone.Worktree()
	require.NoError(t, err)

	// branch with a real change, not merged in the ancestor sense
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/squash-test"),
		Create: true,
	})
	require.NoError(t, err)

	squashFile := filepath.Join(testRepos.cloneDir, "squash.txt")
	err = os.WriteFile(squashFile, []byte("squashed change\n"), 0o644)
	require.NoError(t, err)
	_, err = w.Add("squash.txt")
	require.NoError(t, err)
	_, err = w.Commit("feat: add squash.txt", &git.CommitOptions{})
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	require.NoError(t, err)
	err = os.WriteFile(squashFile, []byte("squashed change\n"), 0o644)
	require.NoError(t, err)
	_, err = w.Add("squash.txt")
	require.NoError(t, err)
	_, err = w.Commit("squash: add squash.txt (#1)", &git.CommitOptions{})
	require.NoError(t, err)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	assert.False(t, gs.BranchIsMerged("tmp/squash-test"))
	assert.True(t, gs.BranchIsSquashMerged("tmp/squash-test"))

	// a branch with a different change is not squash-merged
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/not-squashed"),
		Create: true,
	})
	require.NoError(t, err)
	err = os.WriteFile(squashFile, []byte("different change\n"), 0o644)
	require.NoError(t, err)
	_, err = w.Add("squash.txt")
	require.NoError(t, err)
	_, err = w.Commit("feat: different change", &git.CommitOptions{})
	require.NoError(t, err)

	assert.False(t, gs.BranchIsSquashMerged("tmp/not-squashed"))
}

func TestGitSpruce_DeleteBranch(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	deleted, err := gs.DeleteBranch("tmp/test-one")
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestGitSpruce_DeleteBranchWithForce(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	deleted, err := gs.DeleteBranch("tmp/test-two")
	require.NoError(t, err)
	assert.True(t, deleted)

	branches, _ := testRepos.clone.Branches()
	err = branches.ForEach(func(branch *plumbing.Reference) error {
		if branch.Name().Short() == "tmp/test-two" {
			return errors.New("Branch tmp/test-two still exists in the clone")
		}
		return nil
	})
	require.NoError(t, err)
}

func TestGitSpruce_DeleteBranchWithForceFailsIfBranchCheckedOut(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	w, err := testRepos.clone.Worktree()
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/test-two"),
	})
	require.NoError(t, err)

	deleted, err := gs.DeleteBranch("tmp/test-two")
	require.Error(t, err)
	assert.False(t, deleted)
}

func TestGitSpruce_DeleteBranchWithoutForceRefusesUnmerged(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false

	deleted, err := gs.DeleteBranch("tmp/test-two")
	require.Error(t, err)
	assert.False(t, deleted)
	require.ErrorContains(t, err, "is not merged")

	// the branch must still exist
	_, err = testRepos.clone.Reference(plumbing.NewBranchReferenceName("tmp/test-two"), false)
	require.NoError(t, err)
}

func TestGitSpruce_DeleteBranchWithoutForceDeletesMerged(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false

	deleted, err := gs.DeleteBranch("tmp/test-one")
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestGitSpruce_DeleteBranchWithoutForceDeletesSquashMergedWhenDetected(t *testing.T) {
	testRepos := createTestRepos(t)

	w, err := testRepos.clone.Worktree()
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/squash-test"),
		Create: true,
	})
	require.NoError(t, err)

	squashFile := filepath.Join(testRepos.cloneDir, "squash.txt")
	err = os.WriteFile(squashFile, []byte("squashed change\n"), 0o644)
	require.NoError(t, err)
	_, err = w.Add("squash.txt")
	require.NoError(t, err)
	_, err = w.Commit("feat: add squash.txt", &git.CommitOptions{})
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	require.NoError(t, err)
	err = os.WriteFile(squashFile, []byte("squashed change\n"), 0o644)
	require.NoError(t, err)
	_, err = w.Add("squash.txt")
	require.NoError(t, err)
	_, err = w.Commit("squash: add squash.txt (#1)", &git.CommitOptions{})
	require.NoError(t, err)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false
	gs.DetectSquash = true

	deleted, err := gs.DeleteBranch("tmp/squash-test")
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestGitSpruce_BranchTipHash(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	tip, ok := gs.BranchTipHash("tmp/test-one")
	require.True(t, ok)

	ref, err := testRepos.clone.Reference(plumbing.NewBranchReferenceName("tmp/test-one"), false)
	require.NoError(t, err)
	assert.Equal(t, ref.Hash(), tip)

	_, ok = gs.BranchTipHash("tmp/no-such-branch")
	assert.False(t, ok)
}

func TestGitSpruce_DeleteBranchNotFound(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false

	deleted, err := gs.DeleteBranch("tmp/no-such-branch")
	require.Error(t, err)
	assert.False(t, deleted)
	require.ErrorContains(t, err, "not found")
}

func TestGitSpruce_LoadBranchesMarksGone(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)

	// simulate the upstream branch being deleted: drop the remote-tracking ref
	err = gs.Repo.Storer.RemoveReference(plumbing.NewRemoteReferenceName("origin", "tmp/test-two"))
	require.NoError(t, err)

	branches, err := gs.LoadBranches()
	require.NoError(t, err)
	assert.Contains(t, branches, spruce.GitBranch{Name: "tmp/test-one", IsGone: false})
	assert.Contains(t, branches, spruce.GitBranch{Name: "tmp/test-two", IsGone: true})
}

func TestGitSpruce_DeleteGoneBranchDeletesUnmergedWithoutForce(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false

	// tmp/test-two has a commit not merged into master, but a gone
	// branch is deleted without the merged check
	deleted, err := gs.DeleteGoneBranch("tmp/test-two")
	require.NoError(t, err)
	assert.True(t, deleted)

	_, err = testRepos.clone.Reference(plumbing.NewBranchReferenceName("tmp/test-two"), false)
	assert.Error(t, err)
}

func TestGitSpruce_DeleteGoneBranchFailsIfBranchCheckedOut(t *testing.T) {
	testRepos := createTestRepos(t)

	gs, err := newGitSpruce(testRepos.cloneDir)
	require.NoError(t, err)
	gs.Force = false

	w, err := testRepos.clone.Worktree()
	require.NoError(t, err)

	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("tmp/test-two"),
	})
	require.NoError(t, err)

	deleted, err := gs.DeleteGoneBranch("tmp/test-two")
	require.Error(t, err)
	assert.False(t, deleted)
	require.ErrorContains(t, err, "current branch")
}
