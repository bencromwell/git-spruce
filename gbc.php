#!/usr/bin/env php
<?php

require __DIR__ . '/vendor/autoload.php';

use Cromwell\GitSpruce\CleanBranches;
use Cromwell\GitSpruce\Command\CleanBranchesCommand;
use Cromwell\GitSpruce\Config\LoadConfig;
use Symfony\Component\Console\Application;

$application = new Application();

$config = (new LoadConfig())->load();

$cleanBranchesCommand = new CleanBranchesCommand(
    new CleanBranches($config['merge_base'], $config['ignore_branches'])
);

$application->add($cleanBranchesCommand);
$application->setDefaultCommand($cleanBranchesCommand->getName(), true);

$application->run();
