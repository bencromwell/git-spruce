<?php

declare(strict_types=1);

namespace Cromwell\GitBranchClean;

class GitBranch
{
    private string $name;
    private bool $isGone;

    public function __construct(string $name, bool $isGone)
    {
        $this->name = $name;
        $this->isGone = $isGone;
    }

    public function getName(): string
    {
        return $this->name;
    }

    public function isGone(): bool
    {
        return $this->isGone;
    }
}
