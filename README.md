# git-spruce

[![CI](https://github.com/bencromwell/git-spruce/actions/workflows/ci.yml/badge.svg)](https://github.com/bencromwell/git-spruce/actions/workflows/ci.yml)

Removes local git branches that have been merged into a configured merge base branch. A Go port of [git-spruce-php](https://github.com/bencromwell/git-spruce-php).

Deletion is destructive, so git-spruce asks for confirmation before removing each branch. If you delete a branch by mistake, you can usually recover it from the git `reflog`.

## Usage

```shell
git-spruce [flags]
```

| Flag | Short | Effect |
| --- | --- | --- |
| `--repo` | | Path of the git repository to spruce. Defaults to the working directory. |
| `--prune` | `-p` | Prune stale remote `refs` while fetching. |
| `--squash` | `-s` | Also remove branches whose changes were squash-merged into the merge base. |
| `--yes-all` | `-y` | Skip the per-branch confirmation prompt. |
| `--force` | `-f` | Delete branches even when they are not merged, like `git branch -D`. |
| `--config` | | Path to a `config` file. Defaults to `$HOME/.git-spruce.yaml`. |
| `--version` | | Print the version and exit. |

git-spruce fetches the configured remote before checking branches. Branches with no remote ref, and branches whose remote ref has disappeared, are still checked against the merge base.

## Configuration

git-spruce reads `$HOME/.git-spruce.yaml`, and a `.git-spruce.yaml` in the working directory overrides the global file.

```yaml
merge_base: main
origin: origin
ignore_branches: [develop, main, master]
squash_merged: false
```

| Key | Meaning |
| --- | --- |
| `merge_base` | The branch that merges are checked against. `main` and `master` are treated as synonyms. The default is `main`. |
| `origin` | The remote to fetch and check against. The default is `origin`. |
| `ignore_branches` | Branches never removed, whatever their merge status. Defaults to `develop`, `main` and `master`. |
| `squash_merged` | Also remove branches whose changes landed as a squash merge on the merge base. The default is `false`; `--squash` enables it per run. |

Squash detection compares the diff of each branch against the diff of commits on the merge base, the way `git patch-id` does, so it does not depend on commit messages or ancestry.

## Development

```shell
make build   # build dist/git-spruce
make test    # run the tests
make lint    # golangci-lint
make bench   # run the benchmarks, e.g. make bench BENCH=SprucePipeline
```

## Licence

[MIT](LICENSE)
