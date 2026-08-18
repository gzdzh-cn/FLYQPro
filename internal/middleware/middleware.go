package middleware

import (
	"dzhgo/internal/config"

	"github.com/gogf/gf/v2/frame/g"
)

func init() {

	var s = g.Server()
	//路由鉴权开启
	if config.Cfg.Modules.Base.Middleware.Authority.Enable {

		// 开启跨域
		if config.Cfg.Modules.Base.Middleware.CORS {
			s.BindMiddleware("/app/*", AddonAuthorityMiddleware)
			s.BindMiddleware("/admin/*", AddonAuthorityMiddleware)
		}

		s.BindMiddleware("/app/*/open/*", AppAuthorityMiddlewareOpen) // 开放接口
		// 设备快照和更新事件必须登录；公开的检查、最新版本和下载接口继续免登录。
		s.BindMiddleware("/app/app_update/device/report", AppAuthorityMiddlewareComm)
		s.BindMiddleware("/app/app_update/event/report", AppAuthorityMiddlewareComm)
		s.BindMiddleware("/app/app_update/*", AppAuthorityMiddlewareOpen) // App版本检查、公开下载接口

		s.BindMiddleware("/app/dzh3164/*", AddonAuthorityMiddleware)   // 开放跨域
		s.BindMiddleware("/app/dzh3164/*", AppAuthorityMiddlewareOpen) // 开放接口

		s.BindMiddleware("/app/dict/*", AppAuthorityMiddlewareOpen)   // 开放接口
		s.BindMiddleware("/app/*/comm/*", AppAuthorityMiddlewareComm) // 需登录接口
		s.BindMiddleware("/admin/*/open/*", BaseAuthorityMiddlewareOpen)
		s.BindMiddleware("/admin/*/comm/*", BaseAuthorityMiddlewareComm)

		s.BindMiddleware("/admin/*", BaseAuthorityMiddleware)
		s.BindMiddleware("/app/*", AppAuthorityMiddleware)

		s.BindMiddleware("/admin/*", AutoI18n)  //
		s.BindMiddleware("/admin/*", Exception) //异常抛出捕获
	}

	//请求日志记录到数据库开启
	if config.Cfg.Modules.Base.Middleware.Log.Enable {
		s.BindMiddleware("/admin/*", BaseLog)
		// s.BindMiddleware("/app/*", BaseLog)
	}

}
