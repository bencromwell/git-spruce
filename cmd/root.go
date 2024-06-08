package cmd

import (
	_ "embed"
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
	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Version: fmt.Sprintf("%s (%s)", version, commit),
		Use:     "git-spruce",
		Short:   "Removes branches that have been merged to the configured merge base branch.",
		Long:    "git-spruce " + version + "\n" + helpText,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		Run: execute,
	}

	var (
		cfgFile string
		err     error
	)

	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	// Search config in home directory with name ".git-spruce" (without extension).
	viper.AddConfigPath(home)
	viper.SetConfigType("yaml")
	viper.SetConfigName(".git-spruce")
	// look for config in the working directory
	viper.AddConfigPath(".")

	viper.SetDefault("merge_base", "main")
	viper.SetDefault("origin", "origin")
	viper.SetDefault("ignore_branches", []string{"develop", "main", "master"})

	// viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err = viper.ReadInConfig(); err == nil {
		pterm.Info.Println("Using config file:", viper.ConfigFileUsed())
	}

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.git-spruce.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("prune", "p", false, "Runs prune when fetching.")
	rootCmd.Flags().BoolP("force", "f", false, "Runs git branch -D on detected branches.")
	rootCmd.Flags().BoolP("yes-all", "y", false, "No confirmation for each branch, removes all detected branches.")

	// optional flag for a path to the repo, default to "" or current working directory where you called the command from
	rootCmd.Flags().String("repo", "", "Path of the git repository to spruce.")

	return rootCmd
}

func execute(cmd *cobra.Command, _ []string) {
	// get the config from Viper
	config := viper.GetViper()

	// if they passed --help or -h, exit
	// Cobra will print the help text for us
	if cmd.Flags().Changed("help") || cmd.Flags().Changed("version") {
		os.Exit(0)
	}

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

	// if the user flagged yes to all, we don't need to confirm each branch's deletion
	requireConfirmation := !cmd.Flags().Changed("yes-all")

	totalMerged := 0
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
		if gitSpruce.BranchIsMerged(branch.Name) {
			totalMerged++

			// if they didn't select yes-all ask them to confirm deleting this branch
			canDeleteBranch, canDeleteBranchError := canDeleteBranch(requireConfirmation, branch.Name)

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
		{"Merged", "Not merged", "Removed"},
		{strconv.Itoa(totalMerged), strconv.Itoa(totalNotMerged), strconv.Itoa(totalRemoved)},
	}

	Must(pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render())
}

func canDeleteBranch(requireConfirmation bool, branchName string) (bool, error) {
	if requireConfirmation {
		confirm, err := userConfirmsDeletingBranch(branchName)

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

func userConfirmsDeletingBranch(branchName string) (bool, error) {
	var confirm string
	pterm.Print(pterm.Yellow("Branch " + branchName + " is merged. Remove it? (y/n): "))

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
