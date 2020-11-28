<?php

declare(strict_types=1);

namespace Cromwell\GitSpruce\Config;

use Symfony\Component\Yaml\Yaml;

class LoadConfig
{
    private const CONFIG_FILE = '.gbc.yml';

    public function load(): array
    {
        $config = Yaml::parseFile(__DIR__ . '/../../config.yml');

        if ($this->hasPerRepositoryConfigFile()) {
            $config = array_merge($config, $this->loadRepositoryConfig());
        }

        return $config;
    }

    private function hasPerRepositoryConfigFile(): bool
    {
        return file_exists(self::CONFIG_FILE);
    }

    private function loadRepositoryConfig(): array
    {
        return Yaml::parseFile('gbc.yml');
    }
}
