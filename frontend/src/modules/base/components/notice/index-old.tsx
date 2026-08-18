import {
	ElDialog,
	ElTable,
	ElTableColumn,
	ElPagination,
	ElTag,
	ElButton,
	ElMessageBox,
	ElMessage,
	ElSelect,
	ElOption,
	ElInput,
	ElDatePicker,
	ElDrawer
} from "element-plus";
import { defineComponent, ref, computed, h, onMounted, onUnmounted } from "vue";
import { BellFilled, Search } from "@element-plus/icons-vue";
import "./index.scss";
import { useCool } from "/@/cool";
import dev from "/@/cool/config/dev";

export default defineComponent({
	name: "cl-notice-element",

	props: {},

	emits: ["update:modelValue", "change"],

	setup(props, { emit, expose }) {
		const { service } = useCool();

		// 弹窗显示状态
		const dialogVisible = ref(false);

		// 分页配置
		const currentPage = ref(1);
		const pageSize = ref(10);

		const messageList = ref<any[]>([]);
		const total = ref(0);
		const totalNoRead = ref(0);
		const selectedRows = ref<any[]>([]);
		// 打开通知弹窗
		const open = async () => {
			dialogVisible.value = true;
			emit("change", true);
			refresh();
		};

		const refresh = async () => {
			try {
				const params: any = {
					page: currentPage.value,
					size: pageSize.value,
					noType: noType.value,
					keyWord: keyWord.value
				};

				// 添加日期范围参数
				if (datetimerange.value && datetimerange.value.length === 2) {
					params.startTime = datetimerange.value[0].toISOString();
					params.endTime = datetimerange.value[1].toISOString();
				}

				const { list, pagination } = await service.base.sys.notice.page(params);

				messageList.value = list || [];
				currentPageData.value = list || [];
				total.value = pagination?.total || 0;
				totalNoRead.value = (pagination as any)?.totalNoRead || 0;
			} catch (error) {
				console.error("获取消息列表失败:", error);
				messageList.value = [];
				total.value = 0;
			}
		};

		// 关闭弹窗
		const closeDialog = () => {
			dialogVisible.value = false;
		};

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

		// 删除消息
		const deleteMessage = async (row: any) => {
			try {
				await ElMessageBox.confirm(`确定要删除消息"${row.title}"吗？`, "确认删除", {
					confirmButtonText: "确定",
					cancelButtonText: "取消",
					type: "warning"
				});

				// 调用删除API
				await service.base.sys.notice.delete({
					ids: [row.id]
				});

				ElMessage.success("删除成功");

				// 重新获取消息列表
				await refresh();
			} catch (error) {
				if (error !== "cancel") {
					console.error("删除消息失败:", error);
					ElMessage.error("删除失败");
				}
			}
		};

		// 获取未读消息数量
		const unreadCount = computed(() => {
			if (!messageList.value || !Array.isArray(messageList.value)) {
				return "";
			}
			const unreadItems = totalNoRead.value;

			return unreadItems || "";
		});

		// 当前页的数据（直接使用API返回的数据）
		const currentPageData = ref<any[]>([]);

		// 查询条件
		const noType = ref<string>("");
		const keyWord = ref("");
		const datetimerange = ref<any>(null);
		// 高级搜索弹窗状态
		const advancedSearchVisible = ref(false);

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

		// 触发搜索
		const handleSearch = () => {
			currentPage.value = 1;
			refresh();
		};

		// 重置搜索条件
		const resetSearch = () => {
			noType.value = "";
			keyWord.value = "";
			datetimerange.value = null;
		};

		// 关闭高级搜索弹窗
		const closeAdvancedSearch = () => {
			advancedSearchVisible.value = false;
		};

		// 确认高级搜索
		const confirmAdvancedSearch = () => {
			advancedSearchVisible.value = false;
			handleSearch();
		};

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

		// 处理选择变化
		const handleSelectionChange = (selection: any[]) => {
			selectedRows.value = selection;
		};

		// 批量删除
		const batchDelete = async (ids: string[]) => {
			try {
				// 确认弹窗
				await ElMessageBox.confirm(
					`确定要删除选中的 ${ids.length} 条消息吗？`,
					"确认删除",
					{
						confirmButtonText: "确定",
						cancelButtonText: "取消",
						type: "warning"
					}
				);

				// 执行删除操作
				await service.base.sys.notice.delete({
					ids: ids
				});

				// 成功提醒
				ElMessage.success(`成功删除 ${ids.length} 条消息`);

				// 刷新数据
				refresh();
			} catch (error) {
				if (error !== "cancel") {
					ElMessage.error("删除失败，请重试");
				}
			}
		};

		// 复制到剪贴板
		const copyToClipboard = async (text: string) => {
			try {
				await navigator.clipboard.writeText(text);
				ElMessage.success(`已复制: ${text}`);
			} catch (err) {
				try {
					const ta = document.createElement("textarea");
					ta.value = text;
					document.body.appendChild(ta);
					ta.select();
					document.execCommand("copy");
					document.body.removeChild(ta);
					ElMessage.success(`已复制: ${text}`);
				} catch (e) {
					ElMessage.error("复制失败");
				}
			}
		};
		const permissions = computed(() => {
			return service.base.sys.notice._permission;
		});

		// 格式化时间
		const formatTime = (createTime: string) => {
			return createTime;
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
		expose({
			refresh,
			open
		});

		// 组件挂载时自动获取消息列表
		onMounted(async () => {
			refresh();

			// 启动定时任务
			// taskSetTime();
		});

		// 组件卸载时清理资源
		onUnmounted(() => {
			// sseManager.cleanup();
		});

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
						<div class="message-table-container">
							<div style={{ marginBottom: "16px", textAlign: "right" }}>
								{/* 查询条件：高级搜索按钮 + 关键词 + 搜索按钮 */}
								<div
									style={{
										display: "inline-flex",
										gap: "8px",
										marginRight: "12px",
										verticalAlign: "middle",
										alignItems: "center"
									}}
								>
									<ElButton
										size="small"
										onClick={() => (advancedSearchVisible.value = true)}
									>
										<el-icon style={{ marginRight: "4px" }}>
											<Search />
										</el-icon>
										高级搜索
									</ElButton>
									<ElInput
										placeholder="关键词"
										modelValue={keyWord.value}
										onUpdate:modelValue={(val: string) => (keyWord.value = val)}
										clearable
										size="small"
										onKeydown={(e) => {
											if ((e as KeyboardEvent).key === "Enter") {
												handleSearch();
											}
										}}
										style={{ width: "120px" }}
									/>
									<ElButton type="primary" size="small" onClick={handleSearch}>
										搜索
									</ElButton>
								</div>

								{permissions.value?.delete ? (
									<ElButton
										type="danger"
										size="small"
										onClick={() => {
											if (selectedRows.value.length === 0) {
												ElMessage.warning("请先选择要删除的消息");
												return;
											}
											const ids = selectedRows.value.map(
												(item: any) => item.id
											);
											batchDelete(ids);
										}}
										style={{ marginRight: "10px" }}
										disabled={selectedRows.value.length === 0}
									>
										批量删除 ({selectedRows.value.length})
									</ElButton>
								) : null}
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
							</div>
							<ElTable
								data={messageList.value}
								style={{ width: "100%" }}
								size="small"
								onRow-click={markAsRead}
								onSelection-change={handleSelectionChange}
								rowClassName={({ row }: any) => {
									const isRead = row.status === 1 || row.status === true;
									return isRead ? "read-row" : "unread-row";
								}}
							>
								<ElTableColumn type="selection" width={55} />
								<ElTableColumn
									prop="noType"
									label="类型"
									width={80}
									align="center"
									v-slots={{
										default: ({ row }: any) => {
											const tag = getTypeTag(row.noType);
											const getTypeName = (noType: string) => {
												const typeMap: Record<string, string> = {
													sys: "系统",
													task: "任务",
													approval: "审批",
													meeting: "会议",
													user: "用户",
													work: "工作",
													safe: "安全",
													train: "培训"
												};
												return typeMap[noType] || noType;
											};
											return (
												<ElTag size="small" type={tag.noType}>
													{getTypeName(row.noType)}
												</ElTag>
											);
										}
									}}
								/>
								<ElTableColumn
									prop="title"
									label="标题"
									width={80}
									v-slots={{
										default: ({ row }: any) => {
											const isRead = row.status === 1 || row.status === true;
											return (
												<div class="message-title-cell">
													{row.title}
													{!isRead && <span class="unread-dot"></span>}
												</div>
											);
										}
									}}
								/>
								<ElTableColumn
									prop="remark"
									label="内容"
									minWidth={200}
									showOverflowTooltip
									v-slots={{
										default: ({ row }: any) => {
											const raw = row.remark ?? "";
											const text =
												typeof raw === "string"
													? raw.replace(/<[^>]*>?/g, "")
													: raw;

											let copyNode: any = null;
											if (typeof raw === "string") {
												// 优先匹配独立的 id，其次匹配 guestId
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
										}
									}}
								/>
								<ElTableColumn
									prop="createTime"
									label="时间"
									width={150}
									align="center"
									v-slots={{
										default: ({ row }: any) => formatTime(row.createTime)
									}}
								/>
								<ElTableColumn
									prop="status"
									label="状态"
									width={80}
									align="center"
									v-slots={{
										default: ({ row }: any) => {
											const isRead = row.status === 1 || row.status === true;
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
								/>
								{(() => {
									if (!permissions.value?.delete) {
										return null;
									}

									return (
										<ElTableColumn
											label="操作"
											width={100}
											align="center"
											v-slots={{
												default: ({ row }: any) => {
													if (!permissions.value?.delete) {
														return null;
													}
													return (
														<ElButton
															type="danger"
															size="small"
															onClick={(e: Event) => {
																e.stopPropagation();
																deleteMessage(row);
															}}
														>
															删除
														</ElButton>
													);
												}
											}}
										/>
									);
								})()}
							</ElTable>

							<div class="pagination-container">
								<div class="pagination-container">
									{h(ElPagination, {
										"current-page": currentPage.value,
										"page-size": pageSize.value,
										onCurrentChange: (val: number) => {
											currentPage.value = val;
											// 确保参数更新后再请求数据
											setTimeout(() => refresh(), 0);
										},
										onSizeChange: (val: number) => {
											pageSize.value = val;
											currentPage.value = 1;
											// 确保参数更新后再请求数据
											setTimeout(() => refresh(), 0);
										},
										total: total.value,
										layout: "total, sizes, prev, pager, next, jumper",
										background: true
									})}
								</div>
							</div>
						</div>
					</ElDialog>

					{/* 高级搜索弹窗 */}
					<ElDrawer
						modelValue={advancedSearchVisible.value}
						onUpdate:modelValue={closeAdvancedSearch}
						size="400px"
						direction="rtl"
						destroyOnClose
						v-slots={{
							header: () => (
								<div style={{ fontWeight: "bold", fontSize: "16px" }}>高级搜索</div>
							),
							footer: () => (
								<div style={{ textAlign: "right" }}>
									<ElButton size="default" onClick={resetSearch}>
										重置
									</ElButton>
									<ElButton
										size="default"
										onClick={closeAdvancedSearch}
										style={{ marginLeft: "8px" }}
									>
										取消
									</ElButton>
									<ElButton
										type="primary"
										size="default"
										onClick={confirmAdvancedSearch}
										style={{ marginLeft: "8px" }}
									>
										搜索
									</ElButton>
								</div>
							)
						}}
					>
						<div style={{ padding: "16px 20px 20px 20px" }}>
							<div style={{ marginBottom: "16px" }}>
								<div
									style={{
										display: "flex",
										alignItems: "center",
										marginBottom: "8px"
									}}
								>
									<label
										style={{
											width: "60px",
											marginRight: "12px",
											fontSize: "14px",
											fontWeight: "normal"
										}}
									>
										类型
									</label>
									<ElSelect
										placeholder="请选择类型"
										modelValue={noType.value as any}
										onUpdate:modelValue={(val: any) => (noType.value = val)}
										size="default"
										clearable
										style={{ width: "220px" }}
									>
										{noTypeOptions.map((opt) => (
											<ElOption label={opt.label} value={opt.value as any} />
										))}
									</ElSelect>
								</div>
							</div>
							<div style={{ marginBottom: "16px" }}>
								<div
									style={{
										display: "flex",
										alignItems: "center",
										marginBottom: "8px"
									}}
								>
									<label
										style={{
											width: "60px",
											marginRight: "12px",
											fontSize: "14px",
											fontWeight: "normal"
										}}
									>
										日期范围
									</label>
									<ElDatePicker
										modelValue={datetimerange.value}
										onUpdate:modelValue={(val: any) =>
											(datetimerange.value = val)
										}
										type="datetimerange"
										rangeSeparator="至"
										startPlaceholder="开始日期"
										endPlaceholder="结束日期"
										format="YYYY-MM-DD HH:mm:ss"
										valueFormat="YYYY-MM-DD HH:mm:ss"
										size="default"
										style={{ width: "220px" }}
									/>
								</div>
							</div>
						</div>
					</ElDrawer>
				</>
			);
		};
	}
});
