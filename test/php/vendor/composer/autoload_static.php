<?php

// autoload_static.php @generated by Composer

namespace Composer\Autoload;

class ComposerStaticInitd91a89d496a5c099a47239a903c48dc3
{
    public static $prefixLengthsPsr4 = array (
        'R' => 
        array (
            'Routeguide\\' => 11,
        ),
        'G' => 
        array (
            'GPBMetadata\\' => 12,
        ),
    );

    public static $prefixDirsPsr4 = array (
        'Routeguide\\' => 
        array (
            0 => __DIR__ . '/../..' . '/Routeguide',
        ),
        'GPBMetadata\\' => 
        array (
            0 => __DIR__ . '/../..' . '/GPBMetadata',
        ),
    );

    public static function getInitializer(ClassLoader $loader)
    {
        return \Closure::bind(function () use ($loader) {
            $loader->prefixLengthsPsr4 = ComposerStaticInitd91a89d496a5c099a47239a903c48dc3::$prefixLengthsPsr4;
            $loader->prefixDirsPsr4 = ComposerStaticInitd91a89d496a5c099a47239a903c48dc3::$prefixDirsPsr4;

        }, null, ClassLoader::class);
    }
}
