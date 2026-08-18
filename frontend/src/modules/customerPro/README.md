# customerPro 招生管理

## 更新日志
v1.2.14 -日期：2026-04-29
- 修复 subKf.vue 选中星期字段空值处理：后台返回空字符串时 bind 返回空数组，避免 el-checkbox-group 类型错误

v1.2.1 -日期：2026-04-28
- setting-views.vue 公告模块重构：列表显示 title+content，点击弹窗详情，未读数从 unreadCount 读取，标记已读发送 read=1
- setting-views.vue 快捷操作：删除 name 输入框（仅保留数据字段），删除参数改用 ids 数组格式
- setting-views.vue 统计卡片：增加超管权限判断，非超管不显示不请求
- eps.ts 空值保护：e.api 和 e.prefix 增加 null 检查，防止 forEach 崩溃


v1.2.0 -日期：2026-03-07
-  绑定不同的域名，不同的域名显示对应的各自logo

v1.0.41
- 修改UI颜色
- 优化登录中间件逻辑
