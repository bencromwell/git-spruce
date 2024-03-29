<?php

declare(strict_types=1);

namespace Cromwell\GitSpruce;

class GitSpruce
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
        // handle master and main synonymously
        $rootBranchNames = [
            'main',
            'master',
        ];

        if (in_array($this->mergeBase, $rootBranchNames)) {
            foreach ($rootBranchNames as $mergeBase) {
                if ($this->isValidReference($mergeBase) && $this->isMergedCommand($branchName, $mergeBase)) {
                    return true;
                }
            }
        }

        return $this->isMergedCommand($branchName, $this->mergeBase);
    }

    private function isValidReference(string $reference): bool
    {
        exec(sprintf('git rev-parse --quiet --verify %s', $reference), $output, $returnStatus);

        return $returnStatus === 0;
    }

    private function isMergedCommand(string $branchName, string $mergeBase): bool
    {
        exec(sprintf('git merge-base --is-ancestor %s %s', $branchName, $mergeBase), $output, $returnStatus);

        return $returnStatus === 0;
    }

    public function removeBranch(string $branchName, bool $force): array
    {
        $deleteCommand = $force ? 'D' : 'd';

        exec(sprintf('git branch -%s %s 2>&1', $deleteCommand, $branchName), $output);

        return $output ?? [];
    }
}
