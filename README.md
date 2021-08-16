# git-spruce
CLI for sprucing up your local git, cleans out branches that have been merged upstream.
Spruce is a tree, this tool cleans your branches. So it's an hilarious pun.

## usage

```shell script
Description:
  Removes branches that have been merged to the configured merge base branch

Usage:
  [options]

Options:
  -p, --prune           Run a git fetch -p
  -f, --force           Runs git branch -D on detected branches.
```

Clone the repo to wherever you want, and add an alias:

```shell script
$ alias 'git-spruce'='php /path/to/git-spruce/bin/git-spruce.php'
```

It runs contextual to the current working directory and prompts for each branch to potentially remove.

## options

### prune

The `-p` prune option runs `get fetch -p`. 

This is important because otherwise you don't know what branches have been deleted upstream.

It's not enabled by default because it's slower as it contacts the remote.

### force

By default, `git-spruce` runs `git branch -d`. 

If you've not updated your local merge base yet, you may need the force delete option to remove branches that are in fact merged upstream.

## config

A global default config file, `config.yml`, lives alongside the installation.

This contains the following keys:

- `ignore_branches`: an array of branches to never remove. Defaults to develop, main and master.

- `merge_base`: the merge base we're checking again to check what's merged. In a usual git flow workflow this will be develop. The default here is 'main'.

For the merge_base, 'main' and 'master' branches are treated as synonyms to handle the scenario where both conventions are in use.

You can override the config on a per repository basis by adding a `.git-spruce.yml` configuration file to each repo.

Perhaps you only merge to main on a particular repo, or some other branch. This is for that use case.

## interactive

The deletion action is destructive, so it's interactive, prompting for confirmation for every deletion.

If you mess up you can probably recover a deleted branch from `git reflog`.

## recovery

```shell script
$ git-spruce 
Branch feature/foo is merged. Remove? y
Deleted branch feature/foo (was 19968853d).
```

```shell script
$ git reflog | grep 19968853d
19968853d HEAD@{131}: commit: This is a test commit
```

```shell script
$ git checkout 19968853d
Note: switching to '19968853d'.
```
