package main

import (
        "encoding/json"
        "flag"
        "github.com/potix/utils/signal"
        "github.com/potix/utils/server"
        "github.com/potix/utils/configurator"
        "github.com/potix/gpfs/handler"
        "log"
        "log/syslog"
)

type gpfsServerConfig struct {
        Mode        string `toml:"mode"`
        AddrPort    string `toml:"addrPort"`
        TlsCertPath string `toml:"tlsCertPath"`
        TlsKeyPath  string `toml:"tlsKeyPath"`
}

type gpfsHandlerConfig struct {
        ResourcePath        string `toml:"resourcePath"`
        CookieAuthSecret    string `toml:"cookieAuthSecret"`
        CookieEncryptSecret string `toml:"cookieEncryptSecret"`
        CookieDomain        string `toml:"cookieDomain"`
        CookieSecure        bool   `toml:"cookieSecure"`
	RedirectUrl         string `toml:"redirectUrl"`
        RedisAddrPort       string `toml:"redisAddrPort"`
        RedisPassword       string `toml:"redisPassword"`
        RedisDb             int    `toml:"redisDb"`
        YoutubeApiKey       string `toml:"youtubeApiKey"`
        Title               string `toml:"title"`
}

type gpfsLogConfig struct {
        UseSyslog bool `toml:"useSyslog"`
}

type gpfsConfig struct {
        Verbose    bool                  `toml:"verbose"`
        Server     *gpfsServerConfig     `toml:"server"`
        Handler    *gpfsHandlerConfig    `toml:"handler"`
        Log        *gpfsLogConfig        `toml:"log"`
}

type commandArguments struct {
        configFile string
        videoId    string
}

func verboseLoadedConfig(config *gpfsConfig) {
        if !config.Verbose {
                return
        }
        j, err := json.Marshal(config)
        if err != nil {
                log.Printf("can not dump config: %v", err)
                return
        }
        log.Printf("loaded config: %v", string(j))
}

func main() {
        cmdArgs := new(commandArguments)
        flag.StringVar(&cmdArgs.configFile, "config", "./gpfs.conf", "config file")
        flag.StringVar(&cmdArgs.videoId, "videoid", "", "youtube video id")
        flag.Parse()
        cf, err := configurator.NewConfigurator(cmdArgs.configFile)
        if err != nil {
                log.Fatalf("can not create configurator: %v", err)
        }
        var conf gpfsConfig
        err = cf.Load(&conf)
        if err != nil {
                log.Fatalf("can not load config: %v", err)
        }
        if conf.Server == nil || conf.Handler == nil  {
                log.Fatalf("invalid config")
        }
        if conf.Log != nil && conf.Log.UseSyslog {
                logger, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, "aars")
                if err != nil {
                        log.Fatalf("can not create syslog: %v", err)
                }
                log.SetOutput(logger)
        }
        verboseLoadedConfig(&conf)
        hVerboseOpt := handler.Verbose(conf.Verbose)
        hCookieEncryptSecretOpt := handler.CookieEncryptSecret(conf.Handler.CookieEncryptSecret)
        hCookieSecureOpt := handler.CookieSecure(conf.Handler.CookieSecure)
        var hYoutubeVideoOpt handler.Option = nil
        if conf.Handler.YoutubeApiKey != "" && cmdArgs.videoId != "" {
                youtubeApiKeys, err := configurator.LoadSecretFile(conf.Handler.YoutubeApiKey)
                if err != nil {
                        log.Fatalf("can not load youtube api key file %v: %v", conf.Handler.YoutubeApiKey, err)
                }
                if len(youtubeApiKeys) != 1 {
                        log.Fatalf("no google api key")
                }
                hYoutubeVideoOpt = handler.YoutubeVideo(youtubeApiKeys[0], cmdArgs.videoId)
        }
        hRedisPasswordOpt := handler.RedisPassword(conf.Handler.RedisPassword)
        hRedisDbOpt := handler.RedisDb(conf.Handler.RedisDb)
        hTitleOpt := handler.Title(conf.Handler.Title)
        newHandler, err := handler.NewHandler(
                conf.Handler.ResourcePath,
                conf.Handler.CookieAuthSecret,
                conf.Handler.CookieDomain,
                conf.Handler.RedirectUrl,
                conf.Handler.RedisAddrPort,
                hVerboseOpt,
                hCookieEncryptSecretOpt,
                hCookieSecureOpt,
                hYoutubeVideoOpt,
                hRedisPasswordOpt,
                hRedisDbOpt,
                hTitleOpt,
        )
        if err != nil {
                log.Fatalf("can not create handler: %v", err)
        }
        sVerboseOpt := server.HttpServerVerbose(conf.Verbose)
        sTlsOpt := server.HttpServerTls(conf.Server.TlsCertPath, conf.Server.TlsKeyPath)
        sMode := server.HttpServerMode(conf.Server.Mode)
        newServer, err := server.NewHttpServer(
                conf.Server.AddrPort,
                newHandler,
                sVerboseOpt,
                sTlsOpt,
                sMode,
        )
        if err != nil {
                log.Fatalf("can not create server: %v", err)
        }
        err = newServer.Start()
        if err != nil {
                log.Fatalf("can not start server: %v", err)
        }
        signal.SignalWait(nil)
        newServer.Stop()
}

