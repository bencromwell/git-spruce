<?php

declare(strict_types=1);

namespace Cromwell\GitBranchClean;

class CleanBranches
{
    protected string $mergeBase;

    /**
     * @var string[]
     */
    protected array $branchesToIgnore = [];

    /**
     * @param string $mergeBase
     * @param string[] $branchesToIgnore
     */
    public function __construct(string $mergeBase, array $branchesToIgnore = [])
    {
        $this->mergeBase = $mergeBase;
        $this->branchesToIgnore = $branchesToIgnore;
    }

    public function fetchPrune(): void
    {
        shell_exec('git fetch -p');
    }

    /**
     * @return GitBranch[]
     */
    public function loadBranches(): array
    {
        $branches = shell_exec('git branch -v');
        $branches = explode("\n", $branches);

        $results = [];

        foreach ($branches as $branch) {
            $branch = trim($branch);
            $branch = trim($branch, '*');
            $branch = trim($branch);

            if (empty($branch) || strpos($branch, 'detached') !== false) {
                continue;
            }

            $name = explode(' ', $branch)[0];
            $isGone = strpos($branch, 'gone') !== false;

            if (empty($name) || in_array($name, $this->branchesToIgnore)) {
                continue;
            }

            $results[] = new GitBranch($name, $isGone);
        }

        return $results;
    }

    public function branchIsMerged(string $branchName): bool
    {
        exec(sprintf('git merge-base --is-ancestor %s %s', $branchName, $this->mergeBase), $output, $returnStatus);

        return $returnStatus === 0;
    }

    public function removeBranch(string $branchName, bool $force): array
    {
        $deleteCommand = $force ? 'D' : 'd';

        exec(sprintf('git branch -%s %s 2>&1', $deleteCommand, $branchName), $output);

        return $output ?? [];
    }
}
