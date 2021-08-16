<?php

require __DIR__ . '/../vendor/autoload.php';

use Cromwell\GitSpruce\GitSpruce;
use Cromwell\GitSpruce\Command\CleanBranchesCommand;
use Cromwell\GitSpruce\Config\LoadConfig;
use Symfony\Component\Console\Application;

$application = new Application();

$config = (new LoadConfig())->load();

$cleanBranchesCommand = new CleanBranchesCommand(
    new GitSpruce($config['merge_base'], $config['ignore_branches'])
);

$application->add($cleanBranchesCommand);
$application->setDefaultCommand($cleanBranchesCommand->getName(), true);

$application->run();
