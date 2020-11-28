#!/usr/bin/env php
<?php

require __DIR__ . '/vendor/autoload.php';

use Cromwell\GitBranchClean\CleanBranches;
use Cromwell\GitBranchClean\Command\CleanBranchesCommand;
use Cromwell\GitBranchClean\Config\LoadConfig;
use Symfony\Component\Console\Application;

$application = new Application();

$config = (new LoadConfig())->load();

$cleanBranchesCommand = new CleanBranchesCommand(
    new CleanBranches($config['merge_base'], $config['ignore_branches'])
);

$application->add($cleanBranchesCommand);
$application->setDefaultCommand($cleanBranchesCommand->getName(), true);

$application->run();
