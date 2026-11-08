# git-spruce

[![CI](https://github.com/bencromwell/git-spruce/actions/workflows/ci.yml/badge.svg)](https://github.com/bencromwell/git-spruce/actions/workflows/ci.yml)

CLI for sprucing up your local working copy of a repo, cleans out branches that have been merged upstream.
Spruce is a tree, this tool cleans your branches. It is a [pune](https://wiki.lspace.org/Pune) or play on words.

## Usage

```shell
git-spruce 0.0.0-dev
Removes branches that have been merged to the configured merge base branch.

By default the merge base branch is 'main' or 'master' (both are supported).

In global '$HOME/.git-spruce.yaml' or local to your project '.git-spruce.yaml' you can configure the following:

ignore_branches: An array of branches to never remove. Defaults to develop, main and master.
merge_base: The merge base we're checking against to see what's merged. In a usual git-flow workflow this will be develop. The default here is 'main'.
origin: The remote to check against. Defaults to 'origin'.
squash_merged: Also remove branches whose changes were squash-merged into the merge base (detected by matching the branch diff against commits on the base). Defaults to false. Can also be enabled with --squash / -s.
gone: Also remove branches flagged [gone] in 'git branch -vv', whose remote-tracking ref no longer exists. Gone branches are removed without the merged check, since the upstream has already discarded them. Defaults to false. Can also be enabled with --gone / -g.

For the merge_base, 'main' and 'master' branches are treated as synonyms to handle the scenario where both conventions are in use.

You can override the global config on a per repository basis by adding a .git-spruce.yaml configuration file to each repo.

The deletion action is destructive, so it's interactive, prompting for confirmation for every deletion.

If you mess up you can probably recover a deleted branch from git reflog.

Usage:
  git-spruce [flags]

Flags:
      --config string   config file (default is $HOME/.git-spruce.yaml)
  -f, --force           Runs git branch -D on detected branches.
  -g, --gone            Also removes branches flagged [gone] (their remote-tracking ref no longer exists).
  -h, --help            help for git-spruce
  -p, --prune           Runs prune when fetching.
      --repo string     Path of the git repository to spruce.
  -s, --squash          Also removes branches that were squash-merged into the merge base.
  -v, --version         version for git-spruce
  -y, --yes-all         No confirmation for each branch, removes all detected branches.
```

Clone the repo to wherever you want, and build it:

```shell
$ git clone https://github.com/bencromwell/git-spruce && cd git-spruce && make build
```

Add an alias:

```shell
$ alias 'git-spruce'='/path/to/git-spruce/dist/git-spruce'
```

It runs contextual to the current working directory and prompts for each branch to potentially remove.

## Options

By default, `git-spruce` fetches the configured origin before checking branches, so it always knows what has happened upstream.

The `-p` prune option goes one step further and runs `git fetch -p`, removing remote-tracking refs for branches that have been deleted upstream. A fetch on its own leaves those refs in place, so a branch deleted upstream still looks alive locally.

By default, `git-spruce` runs `git branch -d`, refusing to delete branches that are not merged.

If you've not updated your local merge base yet, you may need the force option to remove branches that are in fact merged upstream.

Branches whose changes were squash-merged look unmerged to git: the branch tip is not an ancestor of the merge base.

The `-s` squash option also detects them, by matching the diff of each branch against the diff of commits on the merge base, the way `git patch-id` does.

Branches whose remote-tracking ref has disappeared flag as `[gone]` in `git branch -vv`.

The `-g` gone option also removes those, without the merged check: the upstream has already discarded them.

The `-y` yes-all option removes every detected branch without prompting.

The `--repo` option points git-spruce at a repository elsewhere. The default is the current working directory.

The `--config` option points at a config file. The default is `$HOME/.git-spruce.yaml`.

## Interactive

The deletion action is destructive, so it's interactive, prompting for confirmation for every deletion. The `-y` yes-all option removes every detected branch without prompting.

## Recovery

```shell
$ git-spruce
Branch feature/foo is merged. Remove it? (y/n): y
Branch deleted: feature/foo (was 19968853d)
```

```shell
$ git reflog | grep 19968853d
19968853d HEAD@{131}: commit: This is a test commit
```

```shell
$ git checkout 19968853d
Note: switching to '19968853d'.
```

For deletions whose output you no longer have, `git reflog` lists the tips you had checked out, and works the same way.
