<template>
	<cl-view-group ref="ViewGroup" leftWidth="260px">
		<template #left>
			<sub-tree ref="subTree" @refresh="refresh" />
		</template>
		<template #right>
			<cl-crud ref="Crud">
				<cl-row>
					<cl-refresh-btn />
					<cl-add-btn />
					<cl-multi-delete-btn />
					<el-button
						v-permission="service.customer_pro.project_group.permission.move"
						type="success"
						:disabled="Table?.selection.length == 0"
						@click="toMove()"
						>转移</el-button
					>
					<cl-flex1 />
					<cl-search-key placeholder="搜索客服组名称" />
				</cl-row>

				<cl-row>
					<cl-table ref="Table" :border="false">
						<template #slot-add="{ scope }">
							<el-button
								text
								bg
								type="info"
								@click="openKf(scope.row)"
								v-if="service.customer_pro.kf._permission.list"
								>客服列表</el-button
							>
						</template>
					</cl-table>
				</cl-row>

				<cl-row>
					<cl-flex1 />
					<cl-pagination />
				</cl-row>

				<!-- 新增、编辑 -->
				<cl-upsert ref="Upsert">
					<!-- slot-cluesRule -->
					<template #slot-cluesRule="{ scope }">
						<div class="rule">
							<el-checkbox-group v-model="scope.checkWeekList">
								<template v-for="(item, index) in weeks" :key="index">
									<el-checkbox :label="item" :value="item" />
								</template>
							</el-checkbox-group>
							<div>
								<div>
									<span>每天数量：</span>
									<el-input-number v-model="scope.receivedCluesNum" />
								</div>
								<span class="warning">注意：默认 0 是不限制</span>
							</div>
						</div>
					</template>
				</cl-upsert>

				<!-- 移动 -->
				<project-move :ref="setRefs('projectMove')" />

				<!-- 客服人员列表 -->
				<el-drawer v-model="drawerKf" :title="groupTitle" direction="rtl" size="85%">
					<sub-kf :key="callKey" :groupId="groupId" :projectId="projectId" />
				</el-drawer>
			</cl-crud>
		</template>
	</cl-view-group>
</template>

<script lang="ts" name="customer_pro-project" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { useViewGroup } from "/$/base";
import { ElDrawer, ElMessageBox } from "element-plus";
import SubTree from "../components/project/subTree.vue";
import SubKf from "../components/project/subKf.vue";
import ProjectMove from "../components/project/move.vue";
import { onMounted, ref, watch, computed } from "vue";
import { useBase } from "/$/base";

const { user } = useBase();
const { service, refs, setRefs } = useCool();
const { ViewGroup } = useViewGroup();
const callKey = ref(0); //重新渲染子组件
const groupId = ref(); //组别id
const groupTitle = ref(); // 组别名称
const projectId = ref(); //项目id
const subTree = ref(); //子组件
const weeks = ref(["星期一", "星期二", "星期三", "星期四", "星期五", "星期六", "星期日"]);
const isAdmin = ref(user.info.roleIds?.split(",").includes("1") || false);

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "客服组名称", prop: "name", required: true, component: { name: "el-input" } },
		{ label: "项目", prop: "projectId", required: true, component: { name: "el-select" } },
		() => {
			return {
				label: "线索规则",
				prop: "cluesRule",
				component: { name: "slot-cluesRule" },
				hidden: !isAdmin.value
			};
		},
		{
			label: "选中星期",
			prop: "checkWeekList",
			hook: {
				bind: (value) => {
					const weekMap: Record<string, string> = {
						"1": "星期一",
						"2": "星期二",
						"3": "星期三",
						"4": "星期四",
						"5": "星期五",
						"6": "星期六",
						"7": "星期日"
					};
					return (
						value &&
						value
							.split(",")
							.map(Number)
							.filter(Boolean)
							.map((num: number) => weekMap[num.toString()])
					);
				},
				submit: (value) => {
					const weekMap: Record<string, string> = {
						星期一: "1",
						星期二: "2",
						星期三: "3",
						星期四: "4",
						星期五: "5",
						星期六: "6",
						星期日: "7"
					};
					return value.map((name: string) => weekMap[name]).join(",");
				}
			},
			value: "1,2,3,4,5,6,7"
		},
		{
			label: "接收数量",
			prop: "receivedCluesNum",
			value: 10
		},
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		}
	],
	async onOpen() {
		const projectList = await service.customer_pro.project.list();
		if (projectList.length == 0) {
			ElMessageBox.alert("请先到左侧栏目：院校管理 >> 项目管理，添加项目", "警告", {
				confirmButtonText: "OK",
				callback: () => {
					Upsert.value?.close();
				}
			});
			return;
		}
		Upsert.value?.setOptions(
			"projectId",
			projectList.map((e) => {
				return {
					label: e.name,
					value: e.id
				};
			})
		);

		if (ViewGroup.value?.selected?.id) {
			Upsert.value?.setForm("projectId", ViewGroup.value?.selected?.id);
		}
	}
});
const Table = useTable({
	columns: [
		{ type: "selection" },
		{ label: "客服组名称", prop: "name" },
		{
			label: "每天上限",
			prop: "receivedCluesNum",
			hidden: !isAdmin.value,
			formatter(row: any) {
				return row.receivedCluesNum == 0 ? "不限制" : row.receivedCluesNum;
			}
		},
		{ label: "今天总数", prop: "hasReceive" },
		{ label: "创建时间", prop: "createTime", width: 160 },
		{ type: "op", width: 220, buttons: ["edit", "slot-add"] }
	]
});

// cl-crud 配置
const Crud = useCrud({
	service: service.customer_pro.project_group
});

// 刷新
function refresh(params?: any) {
	Crud.value?.refresh(params);
}

// 移动成员
async function toMove(item?: any) {
	let ids = [];

	if (item) {
		ids = [item.id];
	} else {
		ids = Table.value?.selection.map((e) => e.id) || [];
	}

	refs.projectMove.open(ids);
}

// 打开客服人员列表
const drawerKf = ref(false);
const openKf = (row: any) => {
	drawerKf.value = true;
	callKey.value++;
	groupId.value = row.id;
	groupTitle.value = row.name;
	projectId.value = row.projectId;
};

watch(drawerKf, (v) => {
	if (!drawerKf.value) {
		subTree.value.refresh();
	}
});

// 获取账号信息
const getUserInfo = async () => {
	isAdmin.value = user.info.roleIds?.split(",").includes("1");
	console.log("isAdmin", isAdmin.value);
};

onMounted(async () => {
	await getUserInfo();
});
</script>

<style lang="scss" scoped>
.warning {
	padding: 0 10px;
	color: #d05a5a;
}
</style>
