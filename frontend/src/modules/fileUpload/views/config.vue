<template>
	<cl-crud ref="Crud">
		<div style="padding: 10px 10px 0px 20px; display: flex; flex-wrap: wrap; row-gap: 10px">
			<!-- 刷新按钮 -->
			<cl-refresh-btn />
			<!-- 新增按钮 -->
			<cl-add-btn />
			<!-- 删除按钮 -->
			<cl-multi-delete-btn style="margin-right: 10px" />

			<el-button
				type="warning"
				@click="multiDo('start')"
				:disabled="Crud?.selection.length ? false : true"
				>批量启动</el-button
			>
			<el-button
				type="danger"
				@click="multiDo('stop')"
				:disabled="Crud?.selection.length ? false : true"
				>批量停止</el-button
			>
			<el-checkbox
				v-model="refresh"
				label="定时刷新"
				size="default"
				border
				style="margin-left: 10px"
			/>

			<cl-flex1 />
			<!-- 关键字搜索 -->
			<cl-search-key />
			<!-- 高级按钮 -->
			<cl-adv-btn />
		</div>

		<div class="divider"></div>

		<cl-row>
			<!-- 数据表格 -->
			<cl-table ref="Table" :border="false">
				<template #slot-start="{ scope }">
					<el-popover
						placement="right"
						:width="80"
						trigger="click"
						popper-class="dzh_popover"
					>
						<template #reference>
							<el-button text bg type="success">更多</el-button>
						</template>
						<div class="more">
							<div class="more_btn">
								<el-button
									text
									bg
									type="warning"
									@click="taskDo(scope.row, 'start')"
									>启动</el-button
								>
							</div>
							<div>
								<el-button text bg type="danger" @click="taskDo(scope.row, 'stop')"
									>停止</el-button
								>
							</div>
						</div>
					</el-popover>
				</template>
			</cl-table>
		</cl-row>

		<cl-row>
			<cl-flex1 />
			<!-- 分页控件 -->
			<cl-pagination />
		</cl-row>

		<!-- 新增、编辑 -->
		<cl-upsert ref="Upsert" />
	</cl-crud>
</template>

<script lang="ts" name="fileUpload-config" setup>
import { useCrud, useTable, useUpsert } from "@cool-vue/crud";
import { useCool } from "/@/cool";
import { ElNotification } from "element-plus";
import { ref, watch } from "vue";
import { useFileUpload } from "/$/fileUpload";

const { service } = useCool();
const { config } = useFileUpload();
const refresh = ref(config.refresh);

// cl-upsert 配置
const Upsert = useUpsert({
	items: [
		{ label: "站点名称", prop: "name", required: true, component: { name: "el-input" } },
		{
			label: "站点域名",
			prop: "siteDomain",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "站点编号",
			prop: "itemId",
			required: true,
			component: { name: "el-input-number", props: { min: 0 } }
		},
		{
			label: "ftp地址",
			prop: "ftpHost",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "ftp端口",
			prop: "ftpPort",
			required: true,
			value: 21,
			component: { name: "el-input" }
		},
		{
			label: "ftp账号",
			prop: "ftpUser",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "ftp密码",
			prop: "ftpPassword",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "本地根路径",
			prop: "localRootPath",
			required: true,
			component: { name: "el-input" }
		},
		{
			label: "线上根路径",
			prop: "remoteRoot",
			required: true,
			component: {
				name: "el-input",
				props: {
					placeholder: "不能为空，根目填写 /"
				}
			}
		},
		{
			label: "上传文件夹",
			prop: "localPathList",
			required: true,
			component: {
				name: "el-input",
				props: {
					placeholder: "多个用英文逗号隔开,支持写文件名称，如 /index.html,/html"
				}
			}
		},
		{
			label: "忽略文件",
			prop: "ignoreList",
			component: {
				name: "el-input",
				props: {
					placeholder: "多个用英文逗号隔开, 如 .DS_Store, .htaccess, .gitignore"
				}
			}
		},
		{
			label: "IP白名单",
			prop: "siteIp",
			component: {
				name: "el-input",
				props: {
					placeholder: "多个用英文逗号隔开"
				}
			}
		},
		{
			label: "排序",
			prop: "orderNum",
			value: 99,
			component: { name: "el-input-number", props: { min: 0 } },
			required: true
		},
		{
			label: "备注",
			prop: "remark",
			component: { name: "el-input", props: { type: "textarea", rows: 4 } }
		},
		() => {
			return {
				label: "异常信息",
				prop: "error",
				hidden(option) {
					if (
						Upsert.value?.mode == "update" &&
						option.scope.error !== "" &&
						option.scope.error !== null
					) {
						return false;
					} else {
						return true;
					}
				},
				value: "",
				component: {
					name: "el-input",
					props: { type: "textarea", rows: 4, disabled: true }
				}
			};
		},
		{
			label: "任务状态",
			prop: "status",
			value: 1,
			component: { name: "el-switch", props: { activeValue: 1, inactiveValue: 0 } }
		}
	]
});

// cl-table 配置
const Table = useTable({
	columns: [
		{ type: "selection" },
		{
			label: "标题",
			prop: "name",
			formatter(row, column, value, index) {
				if (row.error != "") {
					return value + " - 异常";
				} else {
					return value;
				}
			}
		},
		{ label: "站点域名", prop: "siteDomain" },
		{ label: "站点编号", prop: "itemId" },
		{ label: "执行开始时间", prop: "uploadStartTime" },
		{ label: "上次结束时间", prop: "uploadEndTime" },
		{
			label: "进度值",
			prop: "percent",
			formatter(row) {
				return row.percent + "%";
			}
		},
		{
			label: "进度状态",
			prop: "processStatus",
			formatter(row) {
				switch (row.processStatus) {
					case 0:
						return "等待执行";
					case 1:
						return "执行中";
					case 2:
						return "执行完成";
					case -1:
						return "手动停止";
					default:
						break;
				}
				return;
			}
		},
		{ label: "任务状态", prop: "status", component: { name: "cl-switch" } },
		{ label: "创建时间", prop: "createTime" },
		{ type: "op", buttons: ["edit", "slot-start"] }
	]
});

// cl-crud 配置
const Crud = useCrud(
	{
		service: service.fileUpload.config
	},
	(app) => {
		app.refresh();
	}
);

// 执行任务
const taskDo = async (row: any, action: string) => {
	if (action == "start") {
		const res = await service.fileUpload.config.startUpload({ itemId: row.itemId });
		if (res == row.itemId) {
			ElNotification({
				title: "提醒",
				message: "任务执行中",
				type: "warning"
			});
			return;
		}
		ElNotification({
			title: "成功",
			message: "任务开始执行",
			type: "success"
		});
	}

	if (action == "stop") {
		service.fileUpload.config
			.stopUploadById({ itemId: row.itemId })
			.then((res) => {
				console.log("停止", res);
				ElNotification({
					title: "成功",
					message: `任务${row.itemId}执行停止`,
					type: "success"
				});
			})
			.catch((e) => {
				ElNotification({
					title: "错误",
					message: e.message,
					type: "error"
				});
			});
	}

	setTimeout(() => {
		Crud.value?.refresh();
	}, 1000);
};

// 监听定时刷新
let timeRefresh: any;
watch(refresh, (n) => {
	config.setConfig(refresh.value);
	if (n) {
		console.log("定时刷新开始");
		timeRefresh = setInterval(() => {
			Crud.value?.refresh();
		}, 5000);
	} else {
		console.log("定时刷新关闭");
		clearInterval(timeRefresh);
	}
});

// 批量任务
const multiDo = (v: string) => {
	if (v == "start") {
		service.fileUpload.config.multiStart({
			itemIds: Crud.value?.selection.map((e) => e.itemId)
		});
		ElNotification({
			title: "成功",
			message: "任务开始",
			type: "success"
		});
	}
	if (v == "stop") {
		service.fileUpload.config.multiStop({
			itemIds: Crud.value?.selection.map((e) => e.itemId)
		});
		ElNotification({
			title: "成功",
			message: "任务停止",
			type: "success"
		});
	}

	setTimeout(() => {
		Crud.value?.refresh();
	}, 1000);
};
</script>
