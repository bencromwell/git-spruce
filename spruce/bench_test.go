package spruce_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bencromwell/git-spruce/spruce"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/require"
)

// buildBenchRepo creates a repo with a base commit on master plus:
// gone (local only), merged (local + origin ref at the base tip),
// unmerged (local only, one commit ahead) and squash-merged branches.
// It returns a GitSpruce over the repo and every branch ref, so a
// benchmark iteration can restore the refs the previous run deleted.
func buildBenchRepo(
	b *testing.B, gone, merged, unmerged, squashed int,
) (*spruce.GitSpruce, map[plumbing.ReferenceName]plumbing.Hash) {
	b.Helper()

	dir := b.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(b, err)

	w, err := repo.Worktree()
	require.NoError(b, err)

	require.NoError(b, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init\n"), 0o644))
	_, err = w.Add("init.txt")
	require.NoError(b, err)
	baseHash, err := w.Commit("init", &git.CommitOptions{})
	require.NoError(b, err)

	// one commit ahead of the base, used for the unmerged branches
	require.NoError(b, os.WriteFile(filepath.Join(dir, "unmerged.txt"), []byte("unmerged\n"), 0o644))
	_, err = w.Add("unmerged.txt")
	require.NoError(b, err)
	unmergedHash, err := w.Commit("unmerged work", &git.CommitOptions{})
	require.NoError(b, err)

	// put master back at the base commit
	masterRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), baseHash)
	require.NoError(b, repo.Storer.SetReference(masterRef))

	// one squash pair: same change on a branch and as a single commit on master
	require.NoError(b, os.WriteFile(filepath.Join(dir, "squashed.txt"), []byte("squashed\n"), 0o644))
	_, err = w.Add("squashed.txt")
	require.NoError(b, err)
	squashHash, err := w.Commit("squash: add squashed.txt (#1)", &git.CommitOptions{})
	require.NoError(b, err)

	// branch tip carries the same diff over the base commit
	srcRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName("squash/src"), baseHash)
	require.NoError(b, repo.Storer.SetReference(srcRef))
	require.NoError(b, w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("squash/src")}))
	require.NoError(b, os.WriteFile(filepath.Join(dir, "squashed.txt"), []byte("squashed\n"), 0644))
	_, err = w.Add("squashed.txt")
	require.NoError(b, err)
	squashBranchHash, err := w.Commit("feat: add squashed.txt", &git.CommitOptions{})
	require.NoError(b, err)
	require.NoError(b, w.Checkout(&git.CheckoutOptions{Branch: plumbing.NewBranchReferenceName("master")}))
	squashedMaster := plumbing.NewHashReference(plumbing.NewBranchReferenceName("master"), squashHash)
	require.NoError(b, repo.Storer.SetReference(squashedMaster))

	//nolint:exhaustive // only the refs under test belong here
	refs := map[plumbing.ReferenceName]plumbing.Hash{
		plumbing.NewBranchReferenceName("master"):           baseHash,
		plumbing.NewRemoteReferenceName("origin", "master"): baseHash,
	}

	addBranch := func(name string, hash plumbing.Hash, withRemote bool) {
		refs[plumbing.NewBranchReferenceName(name)] = hash
		if withRemote {
			refs[plumbing.NewRemoteReferenceName("origin", name)] = hash
		}
	}

	// gone branches carry an upstream config whose remote-tracking ref
	// no longer exists; without it they were never pushed and load as
	// merely unpushed
	cfg, err := repo.Config()
	require.NoError(b, err)

	for i := range gone {
		name := fmt.Sprintf("gone/%d", i)

		addBranch(name, baseHash, false)

		cfg.Branches[name] = &config.Branch{
			Name:   name,
			Remote: "origin",
			Merge:  plumbing.NewBranchReferenceName(name),
		}
	}
	require.NoError(b, repo.Storer.SetConfig(cfg))

	for i := range merged {
		addBranch(fmt.Sprintf("merged/%d", i), baseHash, true)
	}
	for i := range unmerged {
		addBranch(fmt.Sprintf("unmerged/%d", i), unmergedHash, false)
	}
	for i := range squashed {
		addBranch(fmt.Sprintf("squash/%d", i), squashBranchHash, true)
	}

	for name, hash := range refs {
		require.NoError(b, repo.Storer.SetReference(plumbing.NewHashReference(name, hash)))
	}

	gs := &spruce.GitSpruce{
		MergeBase:        "master",
		BranchesToIgnore: []string{"develop", "main", "master"},
		Origin:           "origin",
		Force:            true,
		DetectSquash:     true,
		Repo:             repo,
		RepoPath:         dir,
	}

	return gs, refs
}

func restoreRefs(b *testing.B, gs *spruce.GitSpruce, refs map[plumbing.ReferenceName]plumbing.Hash) {
	b.Helper()
	for name, hash := range refs {
		require.NoError(b, gs.Repo.Storer.SetReference(plumbing.NewHashReference(name, hash)))
	}
}

// spruceBenchIteration runs one full spruce pass: load branches, merged
// and squash checks, delete.
func spruceBenchIteration(b *testing.B, gs *spruce.GitSpruce) {
	b.Helper()

	branches, err := gs.LoadBranches()
	if err != nil {
		b.Fatal(err)
	}

	for _, branch := range branches {
		if !gs.BranchIsMerged(branch.Name) && !gs.BranchIsSquashMerged(branch.Name) {
			continue
		}

		if _, deleteErr := gs.DeleteBranch(branch.Name); deleteErr != nil {
			b.Fatal(deleteErr)
		}
	}
}

// BenchmarkSprucePipeline runs a full spruce (load branches, merged and
// squash checks, delete) against repos with increasing numbers of gone
// branches. Run with: make bench BENCH=SprucePipeline.
func BenchmarkSprucePipeline(b *testing.B) {
	for _, gone := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("gone=%d", gone), func(b *testing.B) {
			gs, refs := buildBenchRepo(b, gone, 100, 100, 10)

			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				restoreRefs(b, gs, refs)
				b.StartTimer()

				spruceBenchIteration(b, gs)
			}
		})
	}
}
