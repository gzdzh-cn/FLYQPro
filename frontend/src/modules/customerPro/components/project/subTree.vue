<template>
	<div class="dept-tree">
		<div class="dept-tree__header">
			<div>项目类型</div>

			<el-button
				size="small"
				@click="refresh"
				:icon="RefreshRight"
				circle
				v-if="service.customer_pro.project._permission.list"
			/>
			<el-button
				size="small"
				@click="add"
				:icon="Plus"
				circle
				v-if="service.customer_pro.project._permission.add"
			/>
			<el-button
				size="small"
				@click="edit"
				:icon="Edit"
				circle
				v-if="service.customer_pro.project._permission.update"
			/>
		</div>

		<div class="dept-tree__container" @contextmenu.stop.prevent="onContextMenu">
			<el-scrollbar>
				<el-tree
					v-loading="loading"
					node-key="id"
					default-expand-all
					:data="list"
					:props="{
						label: 'name'
					}"
					:draggable="isDrag"
					:allow-drag="allowDrag"
					:allow-drop="allowDrop"
					:expand-on-click-node="false"
					@node-contextmenu="onContextMenu"
				>
					<template #default="{ node, data }">
						<div class="dept-tree__node">
							<span
								class="dept-tree__node-label"
								:class="{
									'is-active': data.id == ViewGroup?.selected?.id
								}"
								@click="rowClick(data)"
								>{{ node.label }}
							</span>
							<span
								v-if="browser.isMini"
								class="dept-tree__node-icon"
								@click="onContextMenu($event, data, node)"
							>
								<el-icon>
									<more-filled />
								</el-icon>
							</span>
						</div>
					</template>
				</el-tree>
			</el-scrollbar>
		</div>

		<cl-form ref="Form">
			<template #slot-projectUserId="{ scope }">
				<div style="display: flex; flex-direction: row; justify-content: space-between">
					<el-select
						v-model="scope.projectUserId"
						filterable
						placeholder="选择成员"
						style="width: 80%"
						@change="selectChange"
						:disabled="scope.projectUserId ? true : false"
					>
						<el-option
							v-for="item in selectOptions"
							:key="item.value"
							:label="item.label"
							:value="item.value"
						/>
					</el-select>

					<el-switch
						v-model="kfStatus"
						inline-prompt
						active-text="推送"
						inactive-text="关闭"
						:active-value="statusMap.active"
						:inactive-value="statusMap.inactive"
					/>
				</div>
			</template>
		</cl-form>

		<cl-dialog title="设置项目主管" v-model="projectVisible">
			<project-role :projectRoleItem="projectRoleItem" />
		</cl-dialog>
	</div>
</template>

<script lang="ts" name="project-tree" setup>
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox, MessageParamsWithType } from "element-plus";
import { useCool } from "/@/cool";
import { ContextMenu, useForm } from "@cool-vue/crud";
import { MoreFilled } from "@element-plus/icons-vue";
import { checkPerm, useViewGroup } from "/$/base";
import { deepTree, revDeepTree } from "/@/cool/utils";
import { Plus, Edit, RefreshRight } from "@element-plus/icons-vue";
import ProjectRole from "./projectRole.vue";
const props = defineProps({
	drag: {
		type: Boolean,
		default: true
	},
	level: {
		type: Number,
		default: 99
	},
	orderInfo: Object
});

const emit = defineEmits(["refresh"]);

const { service, browser } = useCool();
const { ViewGroup } = useViewGroup();
const Form = useForm();
const userMap = ref<{ [key: string]: string }>({});
const kfMap = ref<{ [key: string]: number }>({});
const kfStatus = ref(0); //通知状态
const statusMap = ref({
	active: 1,
	inactive: 0
});

// 客服列表下拉
const selectOptions = ref<
	{
		label: string;
		value: string;
	}[]
>([]);
const kf = reactive({
	status: 1
});

// 树形列表
const list = ref<any[]>([
	{
		name: "全部",
		value: "all"
	}
]);

// 加载中
const loading = ref(false);

// 是否能拖动
const isDrag = ref(false);

// 允许托的规则
function allowDrag({ data }: any) {
	return data.parentId;
}

// 允许放的规则
function allowDrop(_: any, dropNode: any) {
	return dropNode.data.parentId;
}

// 刷新
async function refresh() {
	loading.value = true;
	isDrag.value = false;

	await service.customer_pro.project.list().then((res) => {
		list.value = deepTree(res);
		if (!ViewGroup.value?.selected) {
			rowClick();
		} else {
			rowClick(ViewGroup.value?.selected);
		}
	});

	loading.value = false;
}

// 获取 ids
function rowClick(item?: any) {
	if (!item) {
		item = list.value[0];
	}

	if (item) {
		const ids = item.children ? revDeepTree(item.children).map((e) => e.id) : [];
		ids.unshift(item.id);

		// 刷新列表
		emit("refresh", { page: 1, projectIds: ids });

		// 选择
		ViewGroup.value?.select(item);
		ViewGroup.value?.setTitle(`${item.name} - 客服组列表`);
	}
}

// 编辑
const mode = ref<string>("update");
function rowEdit(item: any) {
	const method = item.id ? "update" : "add";
	mode.value = method;
	Form.value?.open({
		title: `${method == "add" ? "新增" : "编辑"}项目`,
		width: "550px",
		props: {
			labelWidth: "100px"
		},
		items: [
			{
				label: "项目名称",
				prop: "name",
				component: {
					name: "el-input"
				},
				required: true
			}
			// {
			// 	label: "",
			// 	prop: "projectUserId",
			// 	component: {
			// 		name: "slot-projectUserId"
			// 	}
			// }
		],
		form: {
			...item
		},
		on: {
			async open(data) {
				// const kfList = await service.customer_pro.kf.getList({
				// 	roleId: "1815245038695747584",
				// 	userId: data.projectUserId,
				// 	mode: method,
				// 	type: "project"
				// });
				// selectOptions.value = kfList.map((e: { name: any; id: any; kfStatus: any }) => {
				// 	userMap.value[e.id] = e.name; //会员id：name集合
				// 	kfMap.value[e.id] = e.kfStatus; //会员id：status集合
				// 	if (method == "update") {
				// 		kfStatus.value = kfMap.value[data.projectUserId];
				// 	}
				// 	if (method == "add") {
				// 		kfStatus.value = 1;
				// 	}
				// 	return {
				// 		label: e.name,
				// 		value: e.id
				// 	};
				// });
			},
			submit(data, { done, close }) {
				service.customer_pro.project[method]({
					id: item.id,
					// projectUserId: data.projectUserId,
					name: data.name
					// orderNum: data.orderNum,
					// userName: userMap.value[data.projectUserId],
					// kfStatus: kfStatus.value
				})
					.then(() => {
						ElMessage.success(
							`${method == "add" ? "新增" : "编辑"}项目 “${data.name}” 成功`
						);
						close();
						refresh();
					})
					.catch((err: { message: MessageParamsWithType }) => {
						ElMessage.error(err.message);
						done();
					});
			}
		}
	});
}

// 删除
function rowDel(item: any) {
	async function del(f: boolean) {
		await service.customer_pro.project
			.delete({
				ids: [item.id],
				deleteUser: f
			})
			.then(() => {
				// 删除当前
				if (ViewGroup.value?.selected?.id == item.id) {
					rowClick();
				}

				if (f) {
					ElMessage.success("删除成功");
				} else {
					ElMessageBox.confirm(
						`“${item.name}” 项目的组别已成功转移到 “${item.parentName}” 项目。`,
						"删除成功"
					);
				}

				refresh();
			})
			.catch((e) => {
				ElMessage.error(e.message);
				return;
			});
	}

	ElMessageBox.confirm(`该操作会删除 “${item.name}” 项目的所有组别，是否确认？`, "提示", {
		type: "warning",
		confirmButtonText: "直接删除",
		cancelButtonText: "关闭",
		distinguishCancelAndClose: true
	}).then(() => {
		del(true);
	});
	// .catch((action) => {
	// 	if (action == "cancel") {
	// 		del(false);
	// 	}
	// });
}

// 排序
// function treeOrder(f: boolean) {
// 	if (f) {
// 		ElMessageBox.confirm("项目架构已发生改变，是否保存？", "提示", {
// 			type: "warning"
// 		})
// 			.then(async () => {
// 				const ids: any[] = [];

// 				function deep(list: any[], pid: any) {
// 					list.forEach((e) => {
// 						e.parentId = pid;
// 						ids.push(e);

// 						if (e.children && isArray(e.children)) {
// 							deep(e.children, e.id);
// 						}
// 					});
// 				}

// 				deep(list.value, null);

// 				await service.customer_pro.project
// 					.order(
// 						ids.map((e, i) => {
// 							return {
// 								id: e.id,
// 								parentId: e.parentId,
// 								orderNum: i
// 							};
// 						})
// 					)
// 					.then(() => {
// 						ElMessage.success("更新排序成功");
// 					})
// 					.catch((err) => {
// 						ElMessage.error(err.message);
// 					});

// 				refresh();
// 				isDrag.value = false;
// 			})
// 			.catch(() => null);
// 	} else {
// 		refresh();
// 	}
// }

// 右键菜单
function onContextMenu(e: any, d?: any, n?: any) {
	if (!d) {
		d = list.value[0] || {};
	}

	// 权限
	const perm = service.customer_pro.project.permission;
	// const kfPerm = service.customer_pro.kf.permission;

	ContextMenu.open(e, {
		list: [
			{
				label: "新增项目",
				hidden: (n && n.level >= props.level) || !checkPerm(perm.add),
				callback(done) {
					rowEdit({
						name: "",
						parentName: d.name,
						parentId: d.id
					});
					done();
				}
			},
			{
				label: "编辑项目",
				hidden: !checkPerm(perm.update),
				callback(done) {
					rowEdit(d);
					done();
				}
			},
			{
				label: "删除项目",
				hidden: !checkPerm(perm.delete),
				callback(done) {
					rowDel(d);
					done();
				}
			},
			{
				label: "设置项目主管",
				hidden: !checkPerm(perm.addProjectRole),
				callback(done) {
					openProjectRole(d);
					done();
				}
			}
		]
	});
}

// 新增项目
function add() {
	rowEdit({
		name: "",
		parentName: undefined,
		parentId: undefined
	});
}

// 编辑项目
async function edit() {
	const item = await service.customer_pro.project.info({ id: ViewGroup.value?.selected?.id });
	// const item = ViewGroup.value?.selected;
	rowEdit(item);
}

interface ProjectRoleItem {
	id: string;
}
// 设置项目主管
const projectVisible = ref(false);
const projectRoleItem = ref<ProjectRoleItem>();
const openProjectRole = (item: any) => {
	projectRoleItem.value = item;
	projectVisible.value = true;
};

// 设置主管
// const setProjectRole = (item: any) => {
// 	service.customer_pro.project
// 		.delProjectRole({
// 			action: false,
// 			id: item.id
// 		})
// 		.then((res) => {
// 			ElMessage.success("删除成功");
// 			refresh();
// 		})
// 		.catch((e) => {
// 			ElMessage.error(e.message);
// 		});
// };

// 选项改变
const selectChange = (e: any) => {
	if (mode.value == "update") {
		kfStatus.value = kfMap.value[e];
	}
};

onMounted(function () {
	refresh();
});

defineExpose({
	refresh
});
</script>

<style lang="scss" scoped>
.dept-tree {
	height: 100%;
	width: 100%;

	:deep(.el-tree-node__label) {
		display: block;
		height: 100%;
		width: 100%;
	}

	&__header {
		display: flex;
		align-items: center;
		height: 40px;
		padding: 0 10px;
		letter-spacing: 1px;
		position: relative;

		div {
			font-size: 14px;
			flex: 1;
			white-space: nowrap;
		}

		i {
			font-size: 18px;
			cursor: pointer;
		}
	}

	&__op {
		display: flex;

		li {
			display: flex;
			justify-content: center;
			align-items: center;
			list-style: none;
			margin-left: 5px;
			padding: 5px;
			cursor: pointer;

			&:not(.no):hover {
				background-color: #eee;
			}
		}
	}

	&__container {
		height: calc(100% - 40px);

		:deep(.el-tree-node__content) {
			height: 36px;
			margin: 0 5px;
		}
	}

	&__node {
		display: flex;
		align-items: center;
		height: 100%;
		width: 100%;
		box-sizing: border-box;

		&-label {
			display: flex;
			align-items: center;
			flex: 1;
			height: 100%;
			font-size: 14px;
			overflow: hidden;
			text-overflow: ellipsis;
			white-space: nowrap;

			&.is-active {
				color: var(--color-primary);
				font-weight: bold;
			}
			.mark {
				font-size: xx-small;
				color: #e87a90;
				padding: 0 3px;
			}
		}

		&-icon {
			display: flex;
			align-items: center;
			justify-content: center;
			background-color: #eee;
			height: 26px;
			width: 26px;
			text-align: center;
			margin-right: 5px;
			border-radius: 5px;
		}
	}
}
</style>
