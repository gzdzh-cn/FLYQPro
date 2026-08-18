<template>
	<!-- 参与人管理弹窗 -->
	<el-dialog
		v-model="visible"
		title="参与人管理"
		width="800px"
		append-to-body
		class="participant-dialog"
		:close-on-click-modal="false"
	>
		<div class="participant-manager" v-loading="loading">
			<!-- 第一行：所有参与人 -->
			<div class="participant-row">
				<span class="row-label">所有参与人</span>
				<div class="participant-list">
					<template v-if="selectedKfList.length">
						<div v-for="kf in selectedKfList" :key="kf.userId" class="participant-item">
							<el-avatar :size="22" :src="''">
								<el-icon><UserFilled /></el-icon>
							</el-avatar>
							<span class="name">{{ kf.name }}</span>
							<el-icon class="remove-icon" @click="removeParticipant(kf.userId)"><Close /></el-icon>
						</div>
					</template>
					<span v-else class="empty-text">暂无参与人</span>
					<span class="add-link" @click="openSelectDialog">
						<el-icon><Plus /></el-icon> 选择人员
					</span>
				</div>
			</div>
		</div>

		<template #footer>
			<el-button @click="visible = false">取消</el-button>
			<el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
		</template>
	</el-dialog>

	<!-- 选择人员弹窗 -->
	<el-dialog
		v-model="selectVisible"
		title="选择人员"
		width="750px"
		append-to-body
		class="select-person-dialog"
		:close-on-click-modal="false"
	>
		<div class="select-person-layout">
			<!-- 左侧：部门列表 -->
			<div class="dept-panel">
				<div class="dept-header">部门</div>
				<el-scrollbar class="dept-list">
					<ul>
						<li
							:class="{ 'is-active': activeGroupId === null }"
							@click="activeGroupId = null"
						>
							全部
						</li>
						<li
							v-for="g in groupList"
							:key="g.id"
							:class="{ 'is-active': activeGroupId === g.id }"
							@click="activeGroupId = g.id"
						>
							{{ g.name }}
						</li>
					</ul>
				</el-scrollbar>
				<div class="dept-footer">
					<span class="edit-link" @click="openGroupEdit">编辑</span>
				</div>
			</div>

			<!-- 中间：客服列表 -->
			<div class="kf-panel">
				<div class="kf-header">客服人员</div>
				<el-scrollbar class="kf-list">
					<el-checkbox-group v-model="tempSelectedIds">
						<div v-for="kf in filteredKfList" :key="kf.userId" class="kf-item">
							<el-checkbox :label="kf.userId">
								<div class="kf-info">
									<el-avatar :size="22" :src="''">
										<el-icon><UserFilled /></el-icon>
									</el-avatar>
									<span class="name">{{ kf.name }}</span>
								</div>
							</el-checkbox>
						</div>
					</el-checkbox-group>
					<el-empty v-if="!filteredKfList.length" :image-size="60" description="暂无客服人员" />
				</el-scrollbar>
			</div>

			<!-- 右侧：已选列表 -->
			<div class="selected-panel">
				<div class="selected-header">已选 ({{ tempSelectedKfList.length }})</div>
				<el-scrollbar class="selected-list">
					<div v-for="kf in tempSelectedKfList" :key="kf.userId" class="selected-item">
						<el-avatar :size="22" :src="''">
							<el-icon><UserFilled /></el-icon>
						</el-avatar>
						<span class="name">{{ kf.name }}</span>
						<el-icon class="remove-icon" @click="removeTempSelected(kf.userId)"><Close /></el-icon>
					</div>
					<el-empty v-if="!tempSelectedKfList.length" :image-size="60" description="暂未选择" />
				</el-scrollbar>
			</div>
		</div>

		<template #footer>
			<el-button @click="selectVisible = false">取消</el-button>
			<el-button type="primary" @click="confirmSelect">确定</el-button>
		</template>
	</el-dialog>

	<!-- 编辑部门弹窗 -->
	<el-dialog
		v-model="groupEditVisible"
		title="编辑部门"
		width="520px"
		append-to-body
		class="group-edit-dialog"
		:close-on-click-modal="false"
	>
		<div class="edit-tip">
			<el-icon><InfoFilled /></el-icon>
			修改名称、新增或删除项，保存后将同步更新
		</div>

		<div class="edit-list">
			<el-scrollbar ref="groupScrollRef" max-height="380px">
				<div
					v-for="(item, idx) in groupItems"
					:key="idx"
					class="edit-item-wrap"
				>
					<div class="edit-item">
						<span class="idx">{{ idx + 1 }}</span>
						<el-input
							v-model="item.name"
							placeholder="请输入部门名称"
							clearable
						/>
						<el-button
							class="del-btn"
							link
							type="danger"
							@click="groupItems.splice(idx, 1)"
						>
							<el-icon><Delete /></el-icon>
						</el-button>
					</div>
				</div>

				<div v-if="!groupItems.length" class="empty-row">暂无数据</div>
			</el-scrollbar>

			<el-button
				class="add-btn"
				plain
				type="primary"
				@click="addGroup"
			>
				<el-icon><Plus /></el-icon> 添加部门
			</el-button>
		</div>

		<template #footer>
			<el-button @click="groupEditVisible = false">取消</el-button>
			<el-button type="primary" :loading="groupSaving" @click="saveGroups">保存</el-button>
		</template>
	</el-dialog>
</template>

<script lang="ts" name="clues-participant-dialog" setup>
import { ref, computed, nextTick } from "vue";
import { ElMessage } from "element-plus";
import { UserFilled, Plus, Close, InfoFilled, Delete } from "@element-plus/icons-vue";
import { useCool } from "/@/cool";

const props = defineProps<{
	cluesId?: string | number;
	cluesInfo?: any;
}>();

const emit = defineEmits<{
	(e: "saved"): void;
}>();

const { service } = useCool();

// ===== 主弹窗状态 =====
const visible = ref(false);
const loading = ref(false);
const saving = ref(false);

// 已选参与人列表（userId 数组）
const selectedUserIds = ref<string[]>([]);
// 打开弹窗时的原始参与人ID列表，用于计算新增/删除
const initSelectedIds = ref<string[]>([]);
// 全部客服列表
const allKfList = ref<any[]>([]);
// 部门列表
const groupList = ref<any[]>([]);

// 已选参与人的详细信息
const selectedKfList = computed(() => {
	return selectedUserIds.value
		.map((uid) => allKfList.value.find((kf) => kf.userId === uid))
		.filter(Boolean) as any[];
});

// ===== 选择人员弹窗 =====
const selectVisible = ref(false);
const activeGroupId = ref<string | null>(null);
// 临时选中（多选 checkbox）
const tempSelectedIds = ref<string[]>([]);

// 根据部门筛选客服列表（过滤掉负责人）
const filteredKfList = computed(() => {
	const ownerId = String(props.cluesInfo?.servicesId || "");
	let list = allKfList.value;
	if (ownerId) {
		list = list.filter((kf) => String(kf.userId) !== ownerId);
	}
	if (activeGroupId.value === null) return list;
	return list.filter((kf) => kf.groupId === activeGroupId.value);
});

// 临时选中的客服详细信息
const tempSelectedKfList = computed(() => {
	return tempSelectedIds.value
		.map((uid) => allKfList.value.find((kf) => kf.userId === uid))
		.filter(Boolean) as any[];
});

function removeTempSelected(userId: string) {
	const idx = tempSelectedIds.value.indexOf(userId);
	if (idx > -1) tempSelectedIds.value.splice(idx, 1);
}

function openSelectDialog() {
	tempSelectedIds.value = [...selectedUserIds.value];
	selectVisible.value = true;
}

function confirmSelect() {
	selectedUserIds.value = [...tempSelectedIds.value];
	selectVisible.value = false;
}

// ===== 编辑部门弹窗 =====
const groupEditVisible = ref(false);
const groupSaving = ref(false);
const groupItems = ref<{ id?: any; name: string }[]>([]);
const groupScrollRef = ref<any>();

function openGroupEdit() {
	groupItems.value = groupList.value.map((g) => ({
		id: g.id,
		name: g.name
	}));
	groupEditVisible.value = true;
}

function addGroup() {
	groupItems.value.push({ name: "" });
	nextTick(() => {
		const wrap = groupScrollRef.value?.wrapRef as HTMLElement | undefined;
		if (wrap) wrap.scrollTop = wrap.scrollHeight;
	});
}

async function saveGroups() {
	const validItems = groupItems.value.filter((it) => (it.name || "").trim());
	const names = validItems.map((it) => (it.name || "").trim());

	// 校验重名
	const nameSet = new Set<string>();
	for (const n of names) {
		if (nameSet.has(n)) {
			ElMessage.error(`部门名称重复：${n}`);
			return;
		}
		nameSet.add(n);
	}

	groupSaving.value = true;
	try {
		const oldList = [...groupList.value];
		const oldMap = new Map(oldList.map((g) => [g.id, g]));

		// 删除：旧 id 不在新列表里
		const validIds = new Set(validItems.filter((it) => it.id).map((it) => it.id));
		const toDelete = oldList.filter((g) => !validIds.has(g.id));
		if (toDelete.length) {
			await service.customer_pro.project_group.delete({ ids: toDelete.map((g) => g.id) });
		}

		// 遍历有效项，新增或更新
		for (const item of validItems) {
			const name = (item.name || "").trim();
			if (item.id && oldMap.has(item.id)) {
				const old = oldMap.get(item.id);
				if (old.name !== name) {
					await service.customer_pro.project_group.update({ id: item.id, name });
				}
			} else {
				await service.customer_pro.project_group.add({ name, projectId: props.cluesInfo?.projectId });
			}
		}

		ElMessage.success("保存成功");
		groupEditVisible.value = false;
		await loadGroupList();
	} catch (e: any) {
		ElMessage.error(e?.message || "保存失败");
	} finally {
		groupSaving.value = false;
	}
}

// ===== 数据加载 =====
async function loadKfList() {
	try {
		const list = await service.customer_pro.kf.list({
			projectId: props.cluesInfo?.projectId
		});
		allKfList.value = list || [];
	} catch (e) {
		console.error("加载客服列表失败:", e);
		allKfList.value = [];
	}
}

async function loadGroupList() {
	try {
		const list = await service.customer_pro.project_group.list({
			projectId: props.cluesInfo?.projectId
		});
		groupList.value = list || [];
	} catch (e) {
		console.error("加载部门列表失败:", e);
		groupList.value = [];
	}
}

// 从线索数据初始化已选参与人（排除负责人）
function initSelected() {
	const ids = props.cluesInfo?.servicesIds;
	const ownerId = String(props.cluesInfo?.servicesId || "");
	if (!ids) {
		selectedUserIds.value = [];
		initSelectedIds.value = [];
		return;
	}
	const arr: string[] = Array.isArray(ids) ? ids : String(ids).split(",").filter(Boolean);
	const filtered = ownerId ? arr.filter((id) => String(id) !== ownerId) : arr;
	selectedUserIds.value = [...filtered];
	initSelectedIds.value = [...filtered];
}

// ===== 打开弹窗 =====
async function open() {
	visible.value = true;
	loading.value = true;
	try {
		await Promise.all([loadKfList(), loadGroupList()]);
		initSelected();
	} catch (e) {
		console.error("加载数据失败:", e);
	} finally {
		loading.value = false;
	}
}

// 移除参与人
function removeParticipant(userId: string) {
	selectedUserIds.value = selectedUserIds.value.filter((id) => id !== userId);
}

// ===== 保存 =====
async function handleSave() {
	if (!props.cluesId) {
		ElMessage.warning("缺少线索ID");
		return;
	}

	// 计算新增的参与人ID（不在原始列表中的）
	const originalIds = initSelectedIds.value;
	const addedIds = selectedUserIds.value.filter((id) => !originalIds.includes(id));

	saving.value = true;
	try {
		// 如果有新增参与人，调用 addParticipants 接口（会追加 servicesIds + 推送通知）
		if (addedIds.length > 0) {
			await (service.customer_pro.clues as any).addParticipants({
				cluesId: props.cluesId,
				userIds: addedIds
			});
		}

		// 如果有删除的参与人（在原始列表但不在当前列表），直接更新 servicesIds
		const removedIds = originalIds.filter((id) => !selectedUserIds.value.includes(id));
		if (removedIds.length > 0) {
			// 提交时需将负责人ID加回，因为前端已过滤负责人不参与选择
			const ownerId = String(props.cluesInfo?.servicesId || "");
			const finalIds = [...selectedUserIds.value];
			if (ownerId && !finalIds.includes(ownerId)) {
				finalIds.unshift(ownerId);
			}
			const servicesIds = finalIds.length ? finalIds.join(",") : "";
			await service.customer_pro.clues.update({
				id: props.cluesId,
				servicesIds
			});
		}

		// 如果没有任何变化，也允许关闭
		ElMessage.success("保存成功");
		visible.value = false;
		emit("saved");
	} catch (e: any) {
		ElMessage.error(e?.message || "保存失败");
	} finally {
		saving.value = false;
	}
}

defineExpose({ open });
</script>

<style lang="scss">
.participant-dialog {
	border-radius: 12px;
	overflow: hidden;

	.el-dialog__header {
		padding: 16px 20px;
		margin: 0;
		border-bottom: 1px solid #f0f1f5;
		background: linear-gradient(180deg, #fafbfd 0%, #ffffff 100%);

		.el-dialog__title {
			font-size: 15px;
			font-weight: 600;
			color: #1f2329;
		}
	}

	.el-dialog__body {
		padding: 20px !important;
	}

	.el-dialog__footer {
		padding: 12px 20px;
		border-top: 1px solid #f0f1f5;
	}

	.participant-manager {
		.participant-row {
			display: flex;
			align-items: flex-start;
			gap: 12px;

			.row-label {
				flex-shrink: 0;
				width: 80px;
				text-align: right;
				font-size: 13px;
				color: #666;
				line-height: 32px;
			}

			.participant-list {
				flex: 1;
				display: flex;
				flex-wrap: wrap;
				align-items: center;
				gap: 8px;
				min-height: 32px;

				.participant-item {
					display: inline-flex;
					align-items: center;
					gap: 4px;
					padding: 4px 8px;
					background: #f0f7ff;
					border: 1px solid #d9ecff;
					border-radius: 4px;
					font-size: 13px;
					color: #409eff;

					.name {
						max-width: 120px;
						overflow: hidden;
						text-overflow: ellipsis;
						white-space: nowrap;
					}

					.remove-icon {
						cursor: pointer;
						color: #909399;
						font-size: 14px;
						margin-left: 2px;
						transition: color 0.2s;

						&:hover {
							color: #f56c6c;
						}
					}
				}

				.empty-text {
					color: #a8abb2;
					font-size: 13px;
					font-style: italic;
				}

				.add-link {
					display: inline-flex;
					align-items: center;
					gap: 4px;
					padding: 4px 10px;
					font-size: 13px;
					color: var(--color-primary);
					cursor: pointer;
					border: 1px dashed var(--color-primary);
					border-radius: 4px;
					transition: all 0.2s;

					&:hover {
						background: #f0f7ff;
					}
				}
			}
		}
	}
}

.select-person-dialog {
	border-radius: 12px;
	overflow: hidden;

	.el-dialog__header {
		padding: 16px 20px;
		margin: 0;
		border-bottom: 1px solid #f0f1f5;
		background: linear-gradient(180deg, #fafbfd 0%, #ffffff 100%);

		.el-dialog__title {
			font-size: 15px;
			font-weight: 600;
			color: #1f2329;
		}
	}

	.el-dialog__body {
		padding: 0 !important;
	}

	.el-dialog__footer {
		padding: 12px 20px;
		border-top: 1px solid #f0f1f5;
	}

	.select-person-layout {
		display: flex;
		height: 460px;

		// 左侧：部门列表
		.dept-panel {
			width: 180px;
			min-width: 180px;
			border-right: 1px solid #f0f1f5;
			display: flex;
			flex-direction: column;
			background: #fafbfd;

			.dept-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.dept-list {
				flex: 1;

				ul {
					padding: 10px 8px;
					margin: 0;
					list-style: none;

					li {
						display: flex;
						align-items: center;
						padding: 9px 14px;
						margin-bottom: 4px;
						font-size: 13px;
						color: #4e5969;
						border-radius: 6px;
						cursor: pointer;
						transition: all 0.18s ease;

						&::before {
							content: "";
							display: inline-block;
							width: 4px;
							height: 4px;
							border-radius: 50%;
							background: #c0c4cc;
							margin-right: 10px;
							transition: all 0.18s;
						}

						&:hover {
							background: #f0f4ff;
							color: var(--color-primary);

							&::before {
								background: var(--color-primary);
							}
						}

						&.is-active {
							background: var(--color-primary);
							color: #fff;
							font-weight: 500;
							box-shadow: 0 2px 8px rgba(64, 158, 255, 0.25);

							&::before {
								background: #fff;
							}
						}
					}
				}
			}

			.dept-footer {
				padding: 12px 14px;
				border-top: 1px solid #f0f1f5;
				background: #fff;
				text-align: center;

				.edit-link {
					font-size: 13px;
					color: var(--color-primary);
					cursor: pointer;

					&:hover {
						text-decoration: underline;
					}
				}
			}
		}

		// 中间：客服列表
		.kf-panel {
			flex: 1;
			border-right: 1px solid #f0f1f5;
			display: flex;
			flex-direction: column;
			background: #fff;

			.kf-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.kf-list {
				flex: 1;
				padding: 8px 12px;

				.kf-item {
					padding: 6px 0;

					.kf-info {
						display: inline-flex;
						align-items: center;
						gap: 6px;

						.name {
							font-size: 13px;
							color: #333;
						}
					}
				}
			}
		}

		// 右侧：已选列表
		.selected-panel {
			width: 200px;
			min-width: 200px;
			display: flex;
			flex-direction: column;
			background: #fafbfd;

			.selected-header {
				height: 44px;
				line-height: 44px;
				padding: 0 16px;
				font-size: 13px;
				font-weight: 600;
				color: #1f2329;
				border-bottom: 1px solid #f0f1f5;
			}

			.selected-list {
				flex: 1;
				padding: 8px 12px;

				.selected-item {
					display: flex;
					align-items: center;
					gap: 6px;
					padding: 8px 6px;
					border-bottom: 1px solid #f0f1f5;

					.name {
						flex: 1;
						font-size: 13px;
						color: #333;
						overflow: hidden;
						text-overflow: ellipsis;
						white-space: nowrap;
					}

					.remove-icon {
						flex-shrink: 0;
						cursor: pointer;
						color: #909399;
						font-size: 14px;
						transition: color 0.2s;

						&:hover {
							color: #f56c6c;
						}
					}
				}
			}
		}
	}
}

.group-edit-dialog {
	border-radius: 10px;

	.el-dialog__header {
		padding: 16px 20px;
		margin: 0;
		border-bottom: 1px solid #f0f1f5;
		background: linear-gradient(180deg, #fafbfd 0%, #ffffff 100%);

		.el-dialog__title {
			font-size: 15px;
			font-weight: 600;
			color: #1f2329;
		}
	}

	.el-dialog__body {
		padding: 16px 20px !important;
	}

	.el-dialog__footer {
		padding: 12px 20px;
		border-top: 1px solid #f0f1f5;
	}

	.edit-tip {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: #606266;
		margin-bottom: 14px;
		padding: 8px 12px;
		background: #ecf5ff;
		border: 1px solid #d9ecff;
		border-radius: 6px;

		.el-icon {
			color: var(--color-primary);
			font-size: 14px;
		}
	}

	.edit-list {
		.el-scrollbar {
			padding-right: 4px;
		}

		.edit-item-wrap {
			margin-bottom: 8px;
		}

		.edit-item {
			display: flex;
			align-items: center;
			gap: 8px;
			padding: 4px;
			border-radius: 6px;
			transition: background 0.18s;

			&:hover {
				background: #fafbfd;
			}

			.idx {
				display: inline-flex;
				align-items: center;
				justify-content: center;
				width: 24px;
				height: 24px;
				font-size: 12px;
				font-weight: 500;
				color: #909399;
				background: #f0f2f5;
				border-radius: 50%;
				flex-shrink: 0;
			}

			.el-input {
				flex: 1;

				.el-input__wrapper {
					border-radius: 6px;
				}
			}

			.del-btn {
				flex-shrink: 0;
				padding: 6px;

				.el-icon {
					font-size: 16px;
				}
			}
		}

		.empty-row {
			padding: 30px 0;
			text-align: center;
			color: #a8abb2;
			font-size: 13px;
		}

		.add-btn {
			width: 100%;
			margin-top: 8px;
			height: 36px;
			border-radius: 6px;
			border-style: dashed;
			font-size: 13px;

			.el-icon {
				margin-right: 4px;
			}
		}
	}
}
</style>
