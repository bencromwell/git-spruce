package spruce_test

import (
	"errors"
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
	err := os.Mkdir(tempDir, 0755)
	if err != nil {
		return nil, errors.New("Failed to create temp dir")
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

	assert.IsType(t, &git.Repository{}, clone)
	assert.IsType(t, &git.Repository{}, origin)

	// set the clone's remote origin to the origin repo
	_, err = clone.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{originDir},
	})
	require.NoError(t, err)

	// Create a test file
	testFile := filepath.Join(cloneDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
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

	// originBranches, _ := origin.Branches()
	// err = originBranches.ForEach(func(ref *plumbing.Reference) error {
	// 	t.Logf("Branch: %s", ref.Name().Short())
	// 	return nil
	// })
	// require.NoError(t, err)

	// // Add and commit the test file
	// cmd = exec.Command("git", "add", "test.txt")
	// cmd.Dir = tempDir
	// err = cmd.Run()
	// require.NoError(t, err)

	// cmd = exec.Command("git", "commit", "-m", "Initial commit")
	// cmd.Dir = tempDir
	// err = cmd.Run()
	// require.NoError(t, err)

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
