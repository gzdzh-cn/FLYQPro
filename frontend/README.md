# dzhgo-admin-vue


## 扫码加好友拉群
<img src="dzh/weixin.jpg" alt="Description of image" width="200" height="280">



### github
* 后台项目地址：https://github.com/gzdzh-cn/dzhgo
* 前端项目地址：https://github.com/gzdzh-cn/dzhgo-admin-vue


### gitee
* 后台项目地址：https://gitee.com/gzdzh_cn/dzhgo
* 前端项目地址：https://gitee.com/gzdzh_cn/dzhgo-admin-vue.git



## 计划更新
* -如果菜单在某个权限组里面增加增加一个方法权限，需要手动去角色再打开编辑保存一次才可以，比如菜单目录：通用，里面有权限page和list，添加多一个add后，需要到角色编辑保存一次



## 更新日志

### v1.2.31
-修改左侧菜单默认折叠后，增加高亮

### v1.2.3
- 修改主页UI


### v1.1.1
- elementplus升级到2.7.8

### v1.1.0
- 更新插件目录

### v0.1
- 大改，更换成gf的分层设计目录

### v0.0.95
- 修复超管在用户列表时显示角色名称

### v0.0.94
- 修复角色权限问题
- 初始化数据更新
- 增加版本控制和获取版本接口
- 增加插件市场
- 修复登录失败没有报错
- 修复添加用户失败
- 修复新增角色，角色表写入失败
- 增加service的where条件支持元素长度为1的传入
- 兼容go1.21的包引入
- 更新默认菜单顺序，任务管理和字典管理并入系统管理

### v0.0.93
- 前后端：增加菜单：字段类型
- 后端：为了后期用户自增数据生成的id不要和初始数据的id冲突，自增id修改为雪花id
- 后端：更改为分层模式，主逻辑在internal，插件移动到addons
- 前后：更新了新版UI

### v0.0.92
* 修改后台前端订单流水和前端会员文件路径，放到cms文件夹下
* 修改代理图片前缀为 /dzhimg

### 20230730 -v0.0.9
- 修改核心命名为dzhCore和其他文件命名
-
### 20230730 -v0.0.2
- 增加会员登录几种模式

### 20230528 -v0.0.1
- 完成基本目录架构





## 使用

```
yarn install
yarn run dev
yarn run build
```