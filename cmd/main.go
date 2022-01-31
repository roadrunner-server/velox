package main

import (
	"flag"
	"log"

	"github.com/roadrunner-server/velox"
	"github.com/roadrunner-server/velox/build"
	"github.com/roadrunner-server/velox/github"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func main() {
	var cfg *velox.Config

	pathToConfig := flag.String("config", "plugins.toml", "Path to the velox configuration file with plugins")
	out := flag.String("out", "rr", "Output filename (might be with the path)")

	flag.Parse()

	// the user doesn't provide a path to the config
	if pathToConfig == nil {
		log.Fatalf("path to the config should be provided")
	}

	v := viper.New()
	v.SetConfigFile(*pathToConfig)
	err := v.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = v.Unmarshal(&cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = cfg.Validate()
	if err != nil {
		log.Fatal(err)
	}

	zlog, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		_ = zlog.Sync()
	}()

	rp := github.NewRepoInfo(cfg, zlog)
	path, err := rp.DownloadTemplate(cfg.Roadrunner["ref"])
	if err != nil {
		zlog.Fatal("[DOWNLOAD TEMPLATE]", zap.Error(err))
	}

	pMod, err := rp.GetPluginsModData()
	if err != nil {
		zlog.Fatal("[PLUGINS GET MOD INFO]", zap.Error(err))
	}

	builder := build.NewBuilder(path, pMod, *out, zlog, cfg.Velox["build_args"])

	err = builder.Build()
	if err != nil {
		zlog.Fatal("[BUILD FAILED]", zap.Error(err))
	}

	zlog.Info("[BUILD]", zap.String("build finished w/o errors, path", *out))
}
