<?php

declare(strict_types=1);

namespace Cromwell\GitSpruce\Command;

use Cromwell\GitSpruce\CleanBranches;
use Symfony\Component\Console\Command\Command;
use Symfony\Component\Console\Helper\QuestionHelper;
use Symfony\Component\Console\Helper\Table;
use Symfony\Component\Console\Input\InputInterface;
use Symfony\Component\Console\Input\InputOption;
use Symfony\Component\Console\Output\OutputInterface;
use Symfony\Component\Console\Question\ConfirmationQuestion;

class CleanBranchesCommand extends Command
{
    protected static $defaultName = 'gbc:clean';

    protected CleanBranches $cleanBranches;

    public function __construct(CleanBranches $cleanBranches)
    {
        parent::__construct();

        $this->cleanBranches = $cleanBranches;
    }

    protected function configure()
    {
        $this
            ->setDescription('Removes branches that have been merged to the configured merge base branch')
            ->addOption('prune', 'p', InputOption::VALUE_NONE, 'Run a git fetch -p')
            ->addOption('force', 'f', InputOption::VALUE_NONE, 'Runs git branch -D on detected branches.')
        ;
    }

    protected function execute(InputInterface $input, OutputInterface $output)
    {
        if ($input->getOption('prune')) {
            $this->cleanBranches->fetchPrune();
        }

        $branches = $this->cleanBranches->loadBranches();

        $totalMerged = $totalNotMerged = $totalRemoved = 0;

        /** @var QuestionHelper $helper */
        $helper = $this->getHelper('question');

        foreach ($branches as $branch) {
            $merged = $this->cleanBranches->branchIsMerged($branch->getName());

            if ($merged) {
                $totalMerged++;
                $question = new ConfirmationQuestion(sprintf('Branch <info>%s</info> is merged. Remove? ', $branch->getName()), false);

                if ($helper->ask($input, $output, $question)) {
                    $totalRemoved++;
                    $response = $this->cleanBranches->removeBranch($branch->getName(), $input->getOption('force'));
                    foreach ($response as $line) {
                        $output->writeln(sprintf('<error>%s</error>', $line));
                    }
                }
            } else {
                $totalNotMerged++;
                $output->writeln(sprintf('Branch <comment>%s</comment> is not merged', $branch->getName()));
            }
        }

        $output->writeln('');

        $table = new Table($output);
        $table->setHeaders(['Merged', 'Not Merged', 'Removed']);
        $table->addRow([$totalMerged, $totalNotMerged, $totalRemoved]);
        $table->render();

        return Command::SUCCESS;
    }
}
