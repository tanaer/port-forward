package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"goForward/assets"
	"goForward/conf"
	"goForward/sql"
	"goForward/utils"
)

func Run() {
	// [GIN-debug] [WARNING] Running in "debug" mode. Switch to "release" mode in production.
	//  - using env:   export GIN_MODE=release
	//  - using code:  gin.SetMode(gin.ReleaseMode)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("goForward", store))
	r.Use(checkCookieMiddleware)
	r.SetHTMLTemplate(template.Must(template.New("").Funcs(r.FuncMap).ParseFS(assets.Templates, "templates/*")))
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", gin.H{
			"forwardList": sql.GetForwardList(),
		})
	})
	r.GET("/ban", func(c *gin.Context) {
		c.JSON(200, sql.GetIpBan())
	})
	r.POST("/add", func(c *gin.Context) {
		if c.PostForm("localPort") != "" && c.PostForm("remoteAddr") != "" && c.PostForm("remotePort") != "" && c.PostForm("protocol") != "" {
			outTimeStr := c.PostForm("outTime")
			outTimeInt, err := strconv.Atoi(outTimeStr)
			if err != nil {
				outTimeInt = 5
			}
			f := conf.ConnectionStats{
				LocalPort:  c.PostForm("localPort"),
				RemotePort: c.PostForm("remotePort"),
				RemoteAddr: c.PostForm("remoteAddr"),
				Whitelist:  c.PostForm("whitelist"),
				Blacklist:  c.PostForm("blacklist"),
				OutTime:    outTimeInt,
				Protocol:   c.PostForm("protocol"),
			}
			if utils.AddForward(f) {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加成功",
					"suc": true,
				})
			} else {
				c.HTML(200, "msg.tmpl", gin.H{
					"msg": "添加失败，端口已占用",
					"suc": false,
				})
			}
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "添加失败，表单信息不完整",
				"suc": false,
			})
		}
	})
	r.GET("/do/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		status := false
		if err == nil {
			if f.Status == 0 {
				f.Status = 1
				if len(sql.GetAction()) == 1 {
					c.HTML(200, "msg.tmpl", gin.H{
						"msg": "停止失败，请确保有至少一个转发在运行",
						"suc": false,
					})
					return
				}
			} else {
				f.Status = 0
			}
			status = utils.ExStatus(f)
		}
		if status {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作成功",
				"suc": true,
			})
			return
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "操作失败",
				"suc": false,
			})
			return
		}
	})
	r.GET("/del/:id", func(c *gin.Context) {
		id := c.Param("id")
		intID, err := strconv.Atoi(id)
		f := sql.GetForward(intID)
		if err != nil {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败,ID错误",
				"suc": false,
			})
			return
		}
		if len(sql.GetForwardList()) == 1 {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败，请确保有至少一个转发在运行",
				"suc": false,
			})
			return
		}
		if f.Id != 0 && utils.DelForward(f) {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除成功",
				"suc": true,
			})
		} else {
			c.HTML(200, "msg.tmpl", gin.H{
				"msg": "删除失败",
				"suc": false,
			})
		}
	})
	r.GET("/pwd", func(c *gin.Context) {
		if conf.WebPass == "" {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if authed, ok := sessions.Default(c).Get("authed").(bool); ok && authed {
			c.Redirect(http.StatusFound, "/")
			return
		}
		c.HTML(200, "pwd.tmpl", nil)
	})
	r.POST("/pwd", func(c *gin.Context) {
		if !sql.IpFree(c.ClientIP()) {
			c.HTML(http.StatusOK, "msg.tmpl", gin.H{
				"msg": "IP is Ban",
				"suc": false,
			})
			return
		}
		password := c.PostForm("p")
		session := sessions.Default(c)
		session.Options(sessions.Options{MaxAge: 864000})
		if password != conf.WebPass {
			ban := conf.IpBan{
				Ip:        c.ClientIP(),
				TimeStamp: time.Now().Unix(),
			}
			sql.AddBan(ban)
			session.Delete("authed")
			session.Save()
			c.HTML(http.StatusOK, "msg.tmpl", gin.H{
				"msg": "密码错误",
				"suc": false,
			})
			return
		}
		session.Set("authed", true)
		session.Save()
		c.Redirect(http.StatusFound, "/")
	})
	fmt.Println("Web管理面板端口:" + conf.WebPort)
	r.Run("0.0.0.0:" + conf.WebPort)
}

// 密码验证中间件
func checkCookieMiddleware(c *gin.Context) {
	currenPath := c.Request.URL.Path
	if conf.WebPass == "" {
		c.Next()
		return
	}
	if currenPath == "/pwd" {
		c.Next()
		return
	}
	session := sessions.Default(c)
	if authed, ok := session.Get("authed").(bool); !ok || !authed {
		c.Redirect(http.StatusFound, "/pwd")
		c.Abort()
		return
	}
	c.Next()
}
