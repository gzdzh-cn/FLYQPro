<template>
	<div class="dept-tree">
		<div class="dept-tree__header">
			<div>模型类别</div>

			<ul class="dept-tree__op">
				<li @click="rowEdit({ name: '' })">
					<el-tooltip content="新增">
						<el-icon>
							<plus />
						</el-icon>
					</el-tooltip>
				</li>
				<li @click="refresh()">
					<el-tooltip content="刷新">
						<el-icon>
							<refresh-icon />
						</el-icon>
					</el-tooltip>
				</li>

				<!-- <li v-if="drag && !browser.isMini" @click="isDrag = true">
					<el-tooltip content="拖动排序">
						<el-icon>
							<operation />
						</el-icon>
					</el-tooltip>
				</li> -->

				<li v-show="isDrag" class="no">
					<el-button @click="treeOrder(true)" size="small">保存</el-button>
					<el-button @click="treeOrder(false)" size="small">取消</el-button>
				</li>
			</ul>
		</div>

		<div class="dept-tree__container" @contextmenu.stop.prevent="onContextMenu">
			<el-scrollbar>
				<el-tree v-loading="loading" node-key="id" default-expand-all :data="list" :props="{
					label: 'name'
				}" :draggable="isDrag" :allow-drag="allowDrag" :allow-drop="allowDrop" :expand-on-click-node="false"
					@node-contextmenu="onContextMenu">
					<template #default="{ node, data }">
						<div class="dept-tree__node">
							<span class="dept-tree__node-label" :class="{
								'is-active': data.id == ViewGroup?.selected?.id
							}" @click="rowClick(data)">{{ node.label }}</span>
							<span v-if="browser.isMini" class="dept-tree__node-icon"
								@click="onContextMenu($event, data, node)">
								<el-icon>
									<more-filled />
								</el-icon>
							</span>
						</div>
					</template>
				</el-tree>
			</el-scrollbar>
		</div>

		<cl-form ref="Form" />
	</div>
</template>

<script lang="ts" name="model-tree" setup>
import { onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useCool } from "/@/cool";
import { deepTree, revDeepTree } from "/@/cool/utils";
import { isArray } from "lodash-es";
import { ContextMenu, useForm } from "@cool-vue/crud";
import { Refresh as RefreshIcon, Operation, MoreFilled, Plus } from "@element-plus/icons-vue";
import { checkPerm, useViewGroup } from "/$/base";

const props = defineProps({
	drag: {
		type: Boolean,
		default: true
	},
	level: {
		type: Number,
		default: 99
	}
});

const emit = defineEmits(["list-change", "refresh", "user-add"]);

const { service, browser } = useCool();
const { ViewGroup } = useViewGroup();
const Form = useForm();

// 树形列表
const list = ref<any[]>([]);

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

	await service.cms.model.list().then((res) => {
		list.value = deepTree(res);

		if (!ViewGroup.value?.selected) {
			rowClick();
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
		const typeIds = item.children ? revDeepTree(item.children).map((e) => e.id) : [];
		typeIds.unshift(item.id);

		// 刷新列表
		emit("refresh", { page: 1, typeIds: typeIds });

		// 选择
		ViewGroup.value?.select(item);
		ViewGroup.value?.setTitle(`列表-${item.name}`);
	}
}

// 编辑
function rowEdit(item: any) {
	const method = item.id ? "update" : "add";
	const methodName = item.id ? "编辑" : "新增";

	Form.value?.open({
		title: methodName,
		width: "550px",
		props: {
			labelWidth: "100px"
		},
		items: [
			{
				label: "类别",
				prop: "name",
				component: {
					name: "el-input"
				},
				required: true
			},

			{
				label: "排序",
				prop: "orderNum",
				value: 99,
				component: {
					name: "el-input-number",
					props: {
						"controls-position": "right",
						min: 0,
						max: 100
					}
				}
			}
		],
		form: {
			...item
		},
		on: {
			submit(data, { done, close }) {
				service.cms.model[method]({
					id: item.id,
					parentId: item.parentId,
					name: data.name,
					orderNum: data.orderNum
				})
					.then(() => {
						ElMessage.success(`${methodName} ${data.name} 成功`);
						close();
						refresh();
					})
					.catch((err) => {
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
		await service.cms.model
			.delete({
				ids: [item.id],
				deletePro: f
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
						`“${item.name}” 部门的用户已成功转移到 “${item.parentName}” 部门。`,
						"删除成功"
					);
				}
			});

		refresh();
	}

	ElMessageBox.confirm(`该操作会删除 “${item.name}” 类别下所有数据，是否确认？`, "提示", {
		type: "warning",
		confirmButtonText: "直接删除",
		cancelButtonText: "取消",
		distinguishCancelAndClose: true
	})
		.then(() => {
			del(true);
		})
		.catch((action) => {
			return;
			// if (action == "cancel") {
			// 	del(false);
			// }
		});
}

// 排序
function treeOrder(f: boolean) {
	if (f) {
		ElMessageBox.confirm("已发生改变，是否保存？", "提示", {
			type: "warning"
		})
			.then(async () => {
				const ids: any[] = [];

				function deep(list: any[], pid: any) {
					list.forEach((e) => {
						e.parentId = pid;
						ids.push(e);

						if (e.children && isArray(e.children)) {
							deep(e.children, e.id);
						}
					});
				}

				deep(list.value, null);

				await service.base.sys.department
					.order(
						ids.map((e, i) => {
							return {
								id: e.id,
								parentId: e.parentId,
								orderNum: i
							};
						})
					)
					.then(() => {
						ElMessage.success("更新排序成功");
					})
					.catch((err) => {
						ElMessage.error(err.message);
					});

				refresh();
				isDrag.value = false;
			})
			.catch(() => null);
	} else {
		refresh();
	}
}

// 右键菜单
function onContextMenu(e: any, d?: any, n?: any) {
	if (!d) {
		d = list.value[0] || {};
	}

	// 权限
	const perm = service.cms.model.permission;

	ContextMenu.open(e, {
		list: [
			{
				label: "新增",
				hidden: (n && n.level >= props.level) || !checkPerm(perm.add),
				callback(done) {
					rowEdit({
						title: ""
					});
					done();
				}
			},
			{
				label: "编辑",
				hidden: !checkPerm(perm.update),
				callback(done) {
					rowEdit(d);
					done();
				}
			},
			{
				label: "删除",
				hidden: !checkPerm(perm.delete),
				callback(done) {
					rowDel(d);
					done();
				}
			}
		]
	});
}

onMounted(function () {
	refresh();
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
