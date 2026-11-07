package cmd

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/bencromwell/git-spruce/spruce"
	"github.com/go-git/go-git/v5"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//go:embed help.txt
var helpText string

func NewRootCommand(version, commit string) *cobra.Command {
	rootCmd := &cobra.Command{
		Version: fmt.Sprintf("%s (%s)", version, commit),
		Use:     "git-spruce",
		Short:   "Removes branches that have been merged to the configured merge base branch.",
		Long:    "git-spruce " + version + "\n" + helpText,
		Run:     execute,
	}

	viper.SetDefault("merge_base", "main")
	viper.SetDefault("origin", "origin")
	viper.SetDefault("ignore_branches", []string{"develop", "main", "master"})
	viper.SetDefault("squash_merged", false)

	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.git-spruce.yaml)")

	rootCmd.Flags().BoolP("prune", "p", false, "Runs prune when fetching.")
	rootCmd.Flags().BoolP("force", "f", false, "Runs git branch -D on detected branches.")
	rootCmd.Flags().BoolP("yes-all", "y", false, "No confirmation for each branch, removes all detected branches.")
	rootCmd.Flags().BoolP("squash", "s", false, "Also removes branches that were squash-merged into the merge base.")

	rootCmd.Flags().String("repo", "", "Path of the git repository to spruce.")

	return rootCmd
}

func loadConfig(cmd *cobra.Command) error {
	cfgFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stderr, "DBG cfgFile:", cfgFile)

	// SetConfigName resets an explicitly set config file, so it only
	// runs when searching for the default .git-spruce.yaml
	viper.SetConfigType("yaml")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		// search order: working directory, then home, so a repo-level
		// .git-spruce.yaml overrides the global one when both exist
		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".git-spruce")
	}

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return nil
		}

		return err
	}

	pterm.Info.Println("Using config file:", viper.ConfigFileUsed())

	return nil
}

func execute(cmd *cobra.Command, _ []string) {
	Must(loadConfig(cmd))

	config := viper.GetViper()

	// get the repo path from the flag
	repoPath, _ := cmd.Flags().GetString("repo")

	repo, err := git.PlainOpen(repoPath)
	Must(err)

	gitSpruce := &spruce.GitSpruce{
		MergeBase:        config.GetString("merge_base"),
		BranchesToIgnore: config.GetStringSlice("ignore_branches"),
		Origin:           config.GetString("origin"),
		Force:            cmd.Flags().Changed("force"),
		Repo:             repo,
		RepoPath:         repoPath,
	}

	squashFlag, _ := cmd.Flags().GetBool("squash")
	gitSpruce.DetectSquash = squashFlag || config.GetBool("squash_merged")

	// if the user flagged yes to all, we don't need to confirm each branch's deletion
	requireConfirmation := !cmd.Flags().Changed("yes-all")

	detectSquash := gitSpruce.DetectSquash

	totalMerged := 0
	totalSquashMerged := 0
	totalNotMerged := 0
	totalRemoved := 0

	// @todo fix auth when fetching
	// gitSpruce.Fetch(rootCmd.Flags().Changed("prune"))

	branches, err := gitSpruce.LoadBranches()
	Must(err)

	for _, branch := range branches {
		// @todo add an option to delete gone branches
		// if branch.IsGone {
		// fmt.Println("Branch", branch.Name, "is gone.")
		// continue
		// }
		merged := gitSpruce.BranchIsMerged(branch.Name)
		mergeType := "merged"

		if !merged && detectSquash && gitSpruce.BranchIsSquashMerged(branch.Name) {
			merged = true
			mergeType = "squash-merged"
			totalSquashMerged++
		}

		if merged {
			totalMerged++

			// if they didn't select yes-all ask them to confirm deleting this branch
			canDeleteBranch, canDeleteBranchError := canDeleteBranch(requireConfirmation, branch.Name, mergeType)

			if canDeleteBranchError != nil {
				pterm.Error.Println(canDeleteBranchError)
				continue
			}

			if !canDeleteBranch {
				pterm.Warning.Println("Branch not removed:", branch.Name)
				continue
			}

			deleted, deleteBranchError := gitSpruce.DeleteBranch(branch.Name)
			if !deleted {
				pterm.Error.Printf("Error deleting branch [%s]: %s\n", branch.Name, deleteBranchError)
				continue
			}

			pterm.Success.Println("Branch deleted:", branch.Name)
			totalRemoved++
		} else {
			pterm.Info.Println("Branch is not merged:", branch.Name)
			totalNotMerged++
		}
	}

	tableData := pterm.TableData{
		{"Merged", "Squash merged", "Not merged", "Removed"},
		{strconv.Itoa(totalMerged), strconv.Itoa(totalSquashMerged), strconv.Itoa(totalNotMerged), strconv.Itoa(totalRemoved)},
	}

	Must(pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render())
}

func canDeleteBranch(requireConfirmation bool, branchName, mergeType string) (bool, error) {
	if requireConfirmation {
		confirm, err := userConfirmsDeletingBranch(branchName, mergeType)

		if err != nil {
			return false, fmt.Errorf("error confirming branch deletion: %w", err)
		}

		if !confirm {
			pterm.Warning.Println("Branch not removed:", branchName)
			return false, nil
		}
	}

	return true, nil
}

func userConfirmsDeletingBranch(branchName, mergeType string) (bool, error) {
	var confirm string
	pterm.Print(pterm.Yellow("Branch " + branchName + " is " + mergeType + ". Remove it? (y/n): "))

	_, err := fmt.Scanln(&confirm)
	if err != nil {
		return false, fmt.Errorf("error reading user input: %w", err)
	}

	if confirm != "y" && confirm != "Y" {
		return false, nil
	}

	return true, nil
}

func Must(err error) {
	if err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}
