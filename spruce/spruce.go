package spruce

import (
	"errors"
	"fmt"
	"os/exec"

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

	branchList := []GitBranch{}

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()

		if gs.isBranchIgnored(branchName) {
			return nil
		}

		branch := GitBranch{
			Name:   branchName,
			IsGone: gs.isBranchGone(branchName),
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

func (gs *GitSpruce) isBranchGone(branchName string) bool {
	isGone := true

	refName := plumbing.ReferenceName(
		fmt.Sprintf("refs/remotes/%s/%s", gs.Origin, branchName),
	)

	_, err := gs.Repo.Reference(refName, true)

	if err == nil {
		isGone = false
	}

	return isGone
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
	_, err := gs.Repo.Reference(plumbing.ReferenceName(refName), true)

	return err != nil
}

func (gs *GitSpruce) isMerged(branchName string, base string) bool {
	// equivalent to git merge-base --is-ancestor %s %s
	// https://github.com/go-git/go-git/blob/master/_examples/merge_base/main.go

	cmd := exec.Command("git", "merge-base", "--is-ancestor", branchName, base)
	cmd.Dir = gs.RepoPath
	// run the command and get the exit code
	err := cmd.Run()
	if err == nil {
		return true
	}
	// if the exit code is 1, the branch is not an ancestor
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 1 {
			return false
		}
	}

	return false

	// branchHash, _ := gs.repo.ResolveRevision(plumbing.Revision(branchName))
	// baseHash, _ := gs.repo.ResolveRevision(plumbing.Revision(base))

	// commit, _ := gs.repo.CommitObject(*branchHash)
	// baseCommit, _ := gs.repo.CommitObject(*baseHash)

	// isAncestor, err := commit.IsAncestor(baseCommit)

	// if err != nil {
	// 	return false, err
	// }

	// return isAncestor, nil
}

func (gs *GitSpruce) DeleteBranch(branchName string) (bool, error) {
	var err error
	// equivalent to git branch -d %s
	// ref := plumbing.NewBranchReferenceName(branchName)
	// err := gs.repo.Storer.RemoveReference(ref)
	// if err != nil {
	// 	return false, err
	// }
	// return true, nil

	cmd := exec.Command("git", "branch", "-d", branchName)
	cmd.Dir = gs.RepoPath

	_, err = cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}

	// if it didn't work and the user allowed the use of Force, try again with -D
	if gs.Force {
		cmdForce := exec.Command("git", "branch", "-D", branchName)
		cmdForce.Dir = gs.RepoPath

		_, err = cmdForce.CombinedOutput()
		if err != nil {
			return false, err
		}
	}

	return true, nil
}
