<template>
	<cl-view-group ref="ViewGroup" leftWidth="20%">
		<template #left>
			<dept-tree @refresh="refresh" @user-add="onUserAdd" />
		</template>

		<template #right>
			<cl-crud ref="Crud">
				<cl-row>
					<cl-refresh-btn />
					<cl-add-btn />
					<cl-multi-delete-btn />
					<el-button
						v-permission="service.base.sys.user.permission.move"
						type="success"
						:disabled="Table?.selection.length == 0"
						@click="toMove()"
						>转移</el-button
					>
					<cl-flex1 />
					<cl-search-key placeholder="搜索用户名、姓名" />
				</cl-row>

				<cl-row>
					<cl-table ref="Table">
						<!-- 权限 -->
						<template #column-roleName="{ scope }">
							<template v-if="scope.row.roleName">
								<el-tag
									v-for="(item, index) in scope.row.roleName.split(',')"
									:key="index"
									disable-transitions
									size="small"
									effect="dark"
									style="margin: 2px"
									>{{ item }}
								</el-tag>
							</template>
						</template>

						<!-- 单个转移 -->
						<template #slot-btn="{ scope }">
							<el-button
								v-permission="service.base.sys.user.permission.move"
								text
								bg
								@click="toMove(scope.row)"
								>转移</el-button
							>
						</template>

						<template #slot-delete="{ scope }">
							<el-popconfirm title="确定删除会员吗?" @confirm="userDelete(scope.row)">
								<template #reference>
									<el-button
										v-permission="
											service.base.sys.user.permission.delete &&
											scope.row.roleIdList &&
											scope.row.roleIdList.length > 0 &&
											!scope.row.roleIdList.includes('1')
										"
										text
										bg
										type="danger"
										>删除</el-button
									>
								</template>
							</el-popconfirm>
						</template>
					</cl-table>
				</cl-row>

				<cl-row>
					<cl-flex1 />
					<cl-pagination />
				</cl-row>

				<!-- 新增、编辑 -->
				<cl-upsert ref="Upsert" />

				<!-- 移动 -->
				<dept-move :ref="setRefs('deptMove')" />
			</cl-crud>
		</template>
	</cl-view-group>
</template>

<script lang="ts" name="sys-user" setup>
import { useTable, useUpsert, useCrud } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useBase, checkPerm, useViewGroup } from "/$/base";
import DeptMove from "./components/dept/move.vue";
import DeptTree from "./components/dept/tree.vue";

const { service, refs, setRefs } = useCool();
const { ViewGroup } = useViewGroup();
const { user } = useBase();

const perm = service.base.sys.role.permission;

// cl-upsert 配置
const Upsert = useUpsert({
	dialog: {
		width: "800px"
	},

	items: [
		{
			prop: "headImg",
			label: "头像",
			component: {
				name: "cl-upload",
				props: {
					text: "选择头像"
				}
			}
		},
		() => {
			return {
				prop: "name",
				label: "姓名",
				span: 12,
				required: true,
				component: {
					name: "el-input"
				}
			};
		},
		{
			prop: "nickName",
			label: "昵称",
			required: true,
			span: 12,
			component: {
				name: "el-input"
			}
		},
		{
			prop: "username",
			label: "用户名",
			required: true,
			span: 12,
			component: {
				name: "el-input"
			}
		},
		() => {
			return {
				prop: "password",
				label: "密码",
				span: 12,
				required: Upsert.value?.mode == "add",
				component: {
					name: "el-input",
					props: {
						type: "password",
						clearable: true
					}
				},
				rules: [
					{
						min: 6,
						max: 16,
						message: "密码长度在 6 到 16 个字符"
					}
				]
			};
		},
		() => {
			return {
				prop: "roleIdList",
				label: "角色",
				value: [],
				required: true,
				hidden: ({ scope }) => {
					//超管角色不可更改
					return (
							!checkPerm(perm.list) ||
							(scope.roleIdList &&
								scope.roleIdList.length > 0 &&
								scope.roleIdList.includes("1"))
					);
				},
				component: {
					name: "el-select",
					options: [],
					props: {
						multiple: true,
						"multiple-limit": 3
					}
				}
			};
		},
		{
			prop: "phone",
			label: "手机号码",
			span: 12,
			component: {
				name: "el-input"
			}
		},
		{
			prop: "email",
			label: "邮箱",
			span: 12,
			component: {
				name: "el-input"
			}
		},
		{
			prop: "remark",
			label: "备注",
			component: {
				name: "el-input",
				props: {
					type: "textarea",
					rows: 4
				}
			}
		},
		{
			prop: "status",
			label: "状态",
			value: 1,
			component: {
				name: "el-radio-group",
				options: [
					{
						label: "开启",
						value: 1
					},
					{
						label: "关闭",
						value: 0
					}
				]
			}
		}
	],

	onSubmit(data, { next }) {
		next({
			...data,
			departmentId: ViewGroup.value?.selected?.id
		});
	},

	async onOpen() {
		if (checkPerm(perm.list)) {
			// 设置权限列表
			service.base.sys.role.list().then((res) => {
				Upsert.value?.setOptions(
					"roleIdList",
					res.map((e) => {
						return {
							label: e.name || "",
							value: e.id
						};
					})
				);
			});
		}
	}
});

// cl-table 配置
const Table = useTable({
	columns: [
		{
			type: "selection",
			width: 60
		},
		{
			prop: "headImg",
			label: "头像",
			component: {
				name: "cl-avatar"
			}
		},
		{
			prop: "username",
			label: "用户名",
			minWidth: 150
		},
		{
			prop: "name",
			label: "姓名",
			minWidth: 150
		},
		// {
		// 	prop: "nickName",
		// 	label: "昵称",
		// 	minWidth: 150
		// },
		{
			prop: "departmentName",
			label: "部门名称",
			minWidth: 150
		},
		{
			prop: "roleName",
			label: "角色",
			headerAlign: "center",
			minWidth: 120,
			formatter(row, column, value, index) {
				return user.info?.id == 1 ? "超管" : value;
			}
		},
		{
			prop: "status",
			label: "登录状态",
			minWidth: 120,
			component: {
				name: "cl-switch",
				props: {
					activeValue: 1,
					inactiveValue: 0,
					activeText: "开启",
					inactiveText: "禁用",
					inlinePrompt: true
				}
			}
		},
		{
			prop: "loginIp",
			label: "登录IP",
			minWidth: 150
		},
		{
			prop: "phone",
			label: "手机号码",
			minWidth: 150
		},
		{
			prop: "remark",
			label: "备注",
			minWidth: 150
		},
		{
			prop: "createTime",
			label: "创建时间",
			sortable: "desc",
			minWidth: 160
		},
		{
			type: "op",
			buttons: ["slot-btn", "edit", "slot-delete"],
			width: 240
		}
	]
});
// cl-crud 配置
const Crud = useCrud({
	service: service.base.sys.user
});

// 刷新列表
function refresh(params?: any) {
	Crud.value?.refresh(params);
}

// 新增成员
function onUserAdd({ id }: any) {
	Crud.value?.rowAppend({
		departmentId: id
	});
}

// 移动成员
async function toMove(item?: any) {
	let ids = [];

	if (item) {
		ids = [item.id];
	} else {
		ids = Table.value?.selection.map((e) => e.id) || [];
	}

	refs.deptMove.open(ids);
}

// 删除成员
const userDelete = (item?: any) => {
	let ids = [item.id];
	service.base.sys.user.delete({ ids });
	refresh();
};
</script>
