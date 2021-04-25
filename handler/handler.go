package handler

import (
        "log"
        "fmt"
        "context"
        "time"
        "path"
        "net/http"
        "crypto/sha256"
        "github.com/gin-gonic/gin"
        "github.com/gin-contrib/sessions"
        "github.com/gin-contrib/sessions/cookie"
        "github.com/potix/gpfs/helper"
        "github.com/go-redis/redis/v8"
)

const (
        redisTimeout time.Duration = 10 * time.Second
)

type options struct {
        verbose             bool
        cookieEncryptSecret []byte
        cookieExpire        int
        cookieSecure        bool
        youtubeHelper       *helper.YoutubeHelper
        redisUsername       string
        redisPassword       string
        redisDb             int
        redisPoolSize       int
        title               string
}

func defaultOptions() *options {
        return &options{
                verbose: false,
                cookieEncryptSecret: nil,
                cookieExpire: 3600,
                cookieSecure: true,
                youtubeHelper: nil,
                redisUsername: "",
                redisPassword: "",
                redisDb: 0,
                redisPoolSize: 10,
                title: "AARS",
        }
}

type Option func(*options)

func Verbose(verbose bool) Option {
        return func(opts *options) {
                opts.verbose = verbose
        }
}

func CookieEncryptSecret(cookieEncryptSecret string) Option {
        return func(opts *options) {
                s := sha256.Sum256([]byte(cookieEncryptSecret))
                opts.cookieEncryptSecret = s[:]
        }
}

func CookieExpire(cookieExpire int) Option {
        return func(opts *options) {
                opts.cookieExpire = cookieExpire
        }
}

func CookieSecure(cookieSecure bool) Option {
        return func(opts *options) {
                opts.cookieSecure = cookieSecure
        }
}

func YoutubeHelper(youtubeHelper *helper.YoutubeHelper) Option {
        return func(opts *options) {
                opts.youtubeHelper = youtubeHelper
        }
}

func RedisUsername(redisUsername string) Option {
        return func(opts *options) {
                opts.redisUsername = redisUsername
        }
}

func RedisPassword(redisPassword string) Option {
        return func(opts *options) {
                opts.redisPassword = redisPassword
        }
}

func RedisDb(redisDb int) Option {
        return func(opts *options) {
                opts.redisDb = redisDb
        }
}

func RedisPoolSize(redisPoolSize int) Option {
        return func(opts *options) {
                opts.redisPoolSize = redisPoolSize
        }
}

func Title(title string) Option {
        return func(opts *options) {
                opts.title = title
        }
}

type SessionInfo struct {
        AuthType    string
        UserEmail   string
        UserId      string
        UserName    string
        UserPicture string
        UserAgent   string
        Timestamp   int64
}

type Handler struct {
        verbose          bool
        resourcePath     string
        redirectUrl      string
        cookieExpire     int
        store            cookie.Store
        rdb              *redis.Client
        title            string
}

func (h *Handler) Start() error {
        return nil
}

func (h *Handler) Stop() {
}

func (h *Handler) SetRouting(router *gin.Engine) {
        favicon := path.Join(h.resourcePath, "icon", "favicon.ico")
        js := path.Join(h.resourcePath, "js")
        css := path.Join(h.resourcePath, "css")
        img := path.Join(h.resourcePath, "img")
        font := path.Join(h.resourcePath, "font")
        templatePath := path.Join(h.resourcePath, "template", "*")
        router.LoadHTMLGlob(templatePath)
        router.Use(sessions.Sessions("aars", h.store))
        router.GET("/", h.index)
        router.GET("/index.html", h.index)
        router.StaticFile("/favicon.ico", favicon)
        router.Static("/js", js)
        router.Static("/css", css)
        router.Static("/img", img)
        router.Static("/font", font)
        authRouter := router.Group("/auth")
        authRouter.GET("/logout", h.logout)
}

func (h *Handler) forbidden(c *gin.Context) {
        log.Printf(c.ContentType())
        if c.ContentType() != "application/json" {
                c.AbortWithStatus(403)
        } else {
                c.AbortWithStatusJSON(403, gin.H{ "success": false, "message":"forbidden" })
        }
}

func (h *Handler) indexHtml(c *gin.Context) {
        c.HTML(http.StatusOK, "index.tmpl", gin.H{
                "title": h.title,
        })
}

func (h *Handler) redirectHtml(c *gin.Context) {
        c.HTML(http.StatusOK, "redirect.tmpl", gin.H{
                "title": h.title,
                "redirectUrl": h.redirectUrl,
        })
}

func (h *Handler) index(c *gin.Context) {
        headerUserAgent := c.GetHeader("User-Agent")
        session := sessions.Default(c)
        sessionUserAgent := session.Get("userAgent")
        authType := session.Get("authType")
        sessionId := session.Get("sessionId")
        if headerUserAgent == "" || sessionUserAgent == nil || authType == nil || sessionId == nil {
                log.Printf("luck of parameter (%v, %v, %v, %v)", headerUserAgent, sessionUserAgent, authType, sessionId)
		h.redirectHtml(c)
                return
        }
        if headerUserAgent != sessionUserAgent.(string) {
                log.Printf("user agent mismatch")
		h.redirectHtml(c)
                return
        }
        ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
        defer cancel()
        _, err := h.rdb.Get(ctx, sessionId.(string)).Result()
        if err != nil {
                log.Printf("can not get session information from redis (sessionId = %v)", sessionId.(string))
		h.redirectHtml(c)
                return
        }
        h.indexHtml(c)
}

func (h *Handler) logout(c *gin.Context) {
        session := sessions.Default(c)
        sessionId := session.Get("sessionId")
        if sessionId == nil {
		h.redirectHtml(c)
                return
        }
        session.Clear()
        if err := session.Save(); err != nil {
                log.Printf("can not save session: %v", err)
		h.redirectHtml(c)
                return
        }
        ctx, cancel := context.WithTimeout(context.Background(), redisTimeout)
        defer cancel()
        if err := h.rdb.Del(ctx, sessionId.(string)).Err(); err != nil {
                log.Printf("can not session information from redis: %v", err)
		h.redirectHtml(c)
                return
        }
	h.redirectHtml(c)
        return
}

func NewHandler(resourcePath string, cookieAuthSecret string, cookieDomain string, redirectUrl string, redisAddrPort string, opts ...Option) (*Handler, error) {
        baseOpts := defaultOptions()
        for _, opt := range opts {
                opt(baseOpts)
        }
        s := sha256.Sum256([]byte(cookieAuthSecret))
        var cookieStore cookie.Store
        if baseOpts.cookieEncryptSecret != nil {
                cookieStore = cookie.NewStore(s[:], baseOpts.cookieEncryptSecret)
        } else {
                cookieStore = cookie.NewStore(s[:])
        }
        if baseOpts.youtubeHelper != nil {
                title, err := baseOpts.youtubeHelper.GetVideoTitle()
                if err != nil {
                        return nil, fmt.Errorf("can not get youtube video title: %w", err)
                }
                baseOpts.title = title
        }
        cookieStore.Options(sessions.Options{
                Path:     "/",
                Domain:   cookieDomain,
                MaxAge:   baseOpts.cookieExpire,
                Secure:   baseOpts.cookieSecure,
                HttpOnly: true,
                SameSite: http.SameSiteLaxMode,
        })
        rdb := redis.NewClient(&redis.Options{
                PoolSize: baseOpts.redisPoolSize,
                Addr: redisAddrPort,
                Username: baseOpts.redisUsername,
                Password: baseOpts.redisPassword,
                DB: baseOpts.redisDb,
        })
        handler := &Handler{
                verbose: baseOpts.verbose,
                resourcePath: resourcePath,
                redirectUrl: redirectUrl,
                cookieExpire: baseOpts.cookieExpire,
                store: cookieStore,
                rdb: rdb,
                title: baseOpts.title,
        }
        return handler, nil
}

