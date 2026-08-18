import { ElDialog, ElTag, ElButton, ElMessage, ElMessageBox } from "element-plus";
import {
	defineComponent,
	ref,
	computed,
	h,
	onMounted,
	onUnmounted,
	provide,
	inject,
	nextTick
} from "vue";
import { BellFilled, Search } from "@element-plus/icons-vue";
import "./index.scss";
import { useCool } from "/@/cool";
import { useCrud, useTable, useUpsert, useAdvSearch } from "@cool-vue/crud";
import { useBase } from "/$/base";

export default defineComponent({
	name: "cl-notice",

	props: {},

	emits: ["update:modelValue", "change"],

	setup(props, { emit, expose }) {
		const isTask = true;

		const { service } = useCool();
		const { user } = useBase();

		const global = inject("globalOptions", {}) as any;
		provide("globalOptions", {
			...(global || {}),
			style: { ...(global?.style || {}), size: "small" } // 仅调整当前子树 cl 组件尺寸
		});

		const searchRef = ref();
		// 弹窗显示状态
		const dialogVisible = ref(false);
		const selectedRows = ref<any[]>([]);

		const totalNoRead = ref(0);
		// 打开通知弹窗
		const open = async () => {
			dialogVisible.value = true;
			emit("change", true);
			refresh();
		};

		const refresh = async () => {
			if (!Crud?.value) {
				console.log("Crud 还未挂载，等待...");
				const { pagination } = await service.base.sys.notice.page();
				totalNoRead.value = (pagination as any)?.totalNoRead || 0;
				return;
			}
			if (Crud?.value?.refresh) {
				Crud.value.refresh();
			}
		};

		// 关闭弹窗
		const closeDialog = () => {
			dialogVisible.value = false;
		};

		// 获取未读消息数量
		const unreadCount = computed(() => {
			const unreadItems = totalNoRead.value;
			return unreadItems || "";
		});

		// 线索类型选项（可按实际后端字典替换）
		const noTypeOptions = [
			{ label: "全部", value: "" },
			{ label: "系统", value: "sys" },
			{ label: "任务", value: "task" },
			{ label: "审批", value: "approval" },
			{ label: "会议", value: "meeting" },
			{ label: "用户", value: "user" },
			{ label: "工作", value: "work" },
			{ label: "安全", value: "safe" },
			{ label: "培训", value: "train" }
		];

		// cl-crud（与 feedback 用法保持一致）
		const Crud = useCrud(
			{
				service: service.base.sys.notice,
				onRefresh: async function (params, { render }) {
					params.size = 10;
					const { list, pagination } = await service.base.sys.notice.page(params);
					totalNoRead.value = (pagination as any)?.totalNoRead || 0;
					render(list, pagination);
				}
			},
			(app) => {
				app.refresh();
			}
		);
		// cl-upsert 配置
		const Upsert = useUpsert({
			items: [
				{ label: "标题", prop: "title", span: 12, component: { name: "el-input" } },
				{
					label: "类型",
					prop: "noType",
					span: 12,
					component: { name: "el-select", options: noTypeOptions }
				},
				{
					label: "内容",
					prop: "remark",
					component: {
						name: "el-input",
						props: {
							type: "textarea",
							rows: 10
						}
					}
				}
			]
		});
		// 表格
		const Table = useTable({
			columns: [
				{ type: "selection", width: 30 },
				{ label: "类型", prop: "noType", width: 60, align: "center" },
				{ label: "标题", prop: "title", width: 120 },
				{ label: "内容", prop: "remark", minWidth: 200, showOverflowTooltip: true },
				{ label: "时间", prop: "createTime", width: 130, align: "center" },
				{ label: "状态", prop: "status", width: 60, align: "center" },
				{ type: "op", buttons: ["info", "edit", "delete"], width: 150 }
			]
		});

		const searchStatus = ref(false); // 搜索状态
		// 时间选择器起始
		const defaultTime = new Date();
		const shortcuts = [
			{
				text: "当天",
				value: () => {
					const end = new Date();
					end.setDate(end.getDate() + 1);
					end.setHours(0, 0, 0, 0);
					const start = new Date();
					start.setDate(start.getDate());
					start.setHours(0, 0, 0, 0);
					return [start, end];
				}
			},
			{
				text: "昨天",
				value: () => {
					const end = new Date();
					end.setHours(0, 0, 0, 0);
					const start = new Date();
					start.setDate(start.getDate() - 1);
					start.setHours(0, 0, 0, 0);
					return [start, end];
				}
			},
			{
				text: "最近一周",
				value: () => {
					const end = new Date();
					end.setDate(end.getDate() + 1);
					end.setHours(0, 0, 0, 0);
					const start = new Date();
					start.setDate(start.getDate() - 7);
					start.setHours(0, 0, 0, 0);
					return [start, end];
				}
			},
			{
				text: "最近一个月",
				value: () => {
					const end = new Date();
					end.setDate(end.getDate() + 1);
					end.setHours(0, 0, 0, 0);
					const start = new Date();
					start.setMonth(start.getMonth() - 1);
					start.setHours(0, 0, 0, 0);
					return [start, end];
				}
			},
			{
				text: "最近三个月",
				value: () => {
					const end = new Date();
					end.setDate(end.getDate() + 1);
					end.setHours(0, 0, 0, 0);
					const start = new Date();
					start.setMonth(start.getMonth() - 3);
					start.setHours(0, 0, 0, 0);
					return [start, end];
				}
			}
		];
		// 高级搜索
		const AdvSearch = useAdvSearch({
			items: [
				{
					label: "类型",
					prop: "noType",
					component: { name: "el-select", options: noTypeOptions }
				},
				{
					label: "创建时间",
					prop: "datetimerange",
					component: {
						name: "el-date-picker",
						props: {
							type: "datetimerange",
							shortcuts: shortcuts,
							startPlaceholder: "开始日期",
							endPlaceholder: "结束日期",
							defaultTime: defaultTime,
							value: "YYYY-MM-DD HH:mm",
							valueFormat: "YYYY-MM-DD HH:mm",
							timeFormat: "HH:mm"
						}
					}
				}
			],
			op: ["close", "reset", "search"],
			onSearch(data, { next, close }) {
				next(data);
				searchStatus.value = false;
				searchStatus.value = Object.values(data).some((value) => {
					if (value) return true;
				});
			}
		});

		// 处理选择变化
		const handleSelectionChange = (selection: any[]) => {
			selectedRows.value = selection;
		};

		function copyToClipboard(text: string) {
			if (navigator.clipboard && window.isSecureContext) {
				navigator.clipboard.writeText(text).then(() => {
					ElMessage.success("已复制:" + text);
				});
			} else {
				const textarea = document.createElement("textarea");
				textarea.value = text;
				textarea.style.position = "fixed";
				textarea.style.opacity = "0";
				document.body.appendChild(textarea);
				textarea.focus();
				textarea.select();
				try {
					document.execCommand("copy");
					ElMessage.success("已复制:" + text);
				} finally {
					document.body.removeChild(textarea);
				}
			}
		}

		// 标记消息为已读
		const markAsRead = (row: any) => {
			const isRead = row.status === 1 || row.status === true;
			if (isRead) {
				return;
			}

			service.base.sys.notice
				.update({
					id: row.id,
					status: 1
				})
				.then(() => {
					refresh();
				})
				.catch((error) => {
					console.error("标记消息为已读失败:", error);
				});
		};

		const permissions = computed(() => {
			return service.base.sys.notice._permission;
		});
		// 一键已阅
		const noticeReadAll = async () => {
			try {
				// 确认弹窗
				await ElMessageBox.confirm("确定要将所有未读消息标记为已读吗？", "确认操作", {
					confirmButtonText: "确定",
					cancelButtonText: "取消",
					type: "warning"
				});

				// 执行标记已读操作
				await service.base.sys.notice.noticeReadAll();

				// 成功提醒
				ElMessage.success("所有消息已标记为已读");

				// 刷新数据
				refresh();
			} catch (error) {
				if (error !== "cancel") {
					// 如果不是取消操作，显示错误信息
					console.error("标记已读失败:", error);
					ElMessage.error("操作失败，请重试");
				}
			}
		};
		// 获取消息类型标签样式
		const getTypeTag = (noType: string) => {
			const typeMap: Record<
				string,
				{
					noType: "success" | "info" | "warning" | "danger";
					color: string;
				}
			> = {
				sys: { noType: "info", color: "#909399" },
				task: { noType: "success", color: "#67c23a" },
				approval: { noType: "warning", color: "#e6a23c" },
				meeting: { noType: "info", color: "#909399" },
				user: { noType: "success", color: "#67c23a" },
				work: { noType: "warning", color: "#e6a23c" },
				safe: { noType: "danger", color: "#f56c6c" },
				train: { noType: "info", color: "#909399" }
			};
			return typeMap[noType] || { noType: "info", color: "#909399" };
		};

		const taskSetTime = () => {
			// 设置定时器，每分钟获取一次数据
			setInterval(() => {
				refresh();
			}, 1000 * 60 * 1); //1分钟获取一次
		};

		// 暴露方法给父组件
		expose({ refresh, open });

		// 组件挂载时自动获取消息列表
		onMounted(async () => {
			// 等待下一个 tick 确保 Crud 完全初始化
			await nextTick();
			if (user.token) {
				refresh();
			}

			if (isTask && user.token) {
				// 启动定时任务
				// taskSetTime();
			}
		});

		// 组件卸载时清理资源
		onUnmounted(() => {});

		return () => {
			return (
				<>
					<div class="cl-notice__icon" onClick={open}>
						<el-badge value={unreadCount.value}>
							<el-icon size={15}>
								<BellFilled />
							</el-icon>
						</el-badge>
					</div>

					<ElDialog
						modelValue={dialogVisible.value}
						onUpdate:modelValue={closeDialog}
						title="消息通知"
						width="1000px"
						destroyOnClose
					>
						<div
							class="message-table-container"
							style={{ height: "600px", maxHeight: "80vh" }}
						>
							<cl-crud ref={Crud}>
								<cl-row>
									<cl-refresh-btn />
									<cl-add-btn />
									<cl-multi-delete-btn />
									<cl-flex1 />

									{searchStatus.value && (
										<ElButton type="info" size="small" text bg icon={Search}>
											正在搜索中
										</ElButton>
									)}

									<cl-adv-btn />
									<cl-search-key ref={searchRef} />
									{permissions.value?.noticeReadAll ? (
										<ElButton
											type="primary"
											size="small"
											onClick={noticeReadAll}
											disabled={
												unreadCount.value === 0 || unreadCount.value === ""
											}
										>
											一键已阅
											{Number(unreadCount.value) > 0
												? ` (${unreadCount.value})`
												: ""}
										</ElButton>
									) : null}
								</cl-row>
								<cl-row>
									<cl-table
										ref={Table}
										border={false}
										row-key="id"
										size="small"
										onRow-click={markAsRead}
										onSelection-change={handleSelectionChange}
									>
										{{
											"column-noType": ({ scope }: any) => {
												const tag = getTypeTag(scope.row.noType);
												const map: Record<string, string> = {
													sys: "系统",
													task: "任务",
													approval: "审批",
													meeting: "会议",
													user: "用户",
													work: "工作",
													safe: "安全",
													train: "培训"
												};
												return (
													<ElTag size="small" type={tag.noType}>
														{map[scope.row.noType] || scope.row.noType}
													</ElTag>
												);
											},
											"column-title": ({ scope }: any) => {
												const isRead =
													scope.row.status === 1 ||
													scope.row.status === true;
												return (
													<div class="message-title-cell">
														{scope.row.title}
														{!isRead && (
															<span class="unread-dot"></span>
														)}
													</div>
												);
											},
											"column-remark": ({ scope }: any) => {
												const raw = scope.row.remark ?? "";
												const text =
													typeof raw === "string"
														? raw.replace(/<[^>]*>?/g, "")
														: raw;
												let copyNode: any = null;
												if (typeof raw === "string") {
													const idMatch = raw.match(
														/(?:^|[^A-Za-z0-9_])id[:：]\s*(\d+)/i
													);
													const guestIdMatch =
														raw.match(/guestId[:：]\s*(\d+)/i);
													const toCopy =
														idMatch?.[1] ?? guestIdMatch?.[1] ?? null;
													if (toCopy) {
														copyNode = (
															<ElButton
																style={{ marginRight: "8px" }}
																type="primary"
																size="small"
																onClick={(e: Event) => {
																	e.stopPropagation();
																	copyToClipboard(toCopy);
																}}
															>
																复制
															</ElButton>
														);
													}
												}
												return (
													<div
														style={{
															display: "inline-flex",
															alignItems: "center"
														}}
													>
														{copyNode}
														<span>{text}</span>
													</div>
												);
											},
											"column-status": ({ scope }: any) => {
												const isRead =
													scope.row.status === 1 ||
													scope.row.status === true;
												return (
													<ElTag
														size="small"
														type={isRead ? "success" : "danger"}
													>
														{isRead ? "已读" : "未读"}
													</ElTag>
												);
											}
										}}
									</cl-table>
								</cl-row>

								<cl-row>
									<cl-flex1 />
									<cl-pagination />
								</cl-row>
								<cl-upsert ref={Upsert} />
								<cl-adv-search ref={AdvSearch} />
							</cl-crud>
						</div>
					</ElDialog>
				</>
			);
		};
	}
});
