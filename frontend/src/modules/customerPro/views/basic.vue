<template>
	<div class="view-my">
		<el-form :model="basicForm" label-width="120px">
			<el-tabs type="border-card">
				<el-tab-pane label="基础配置">
					<el-form-item label="搜索引擎开启">
						<el-switch
							v-model="basicForm.esOpen"
							:active-value="1"
							:inactive-value="0"
							active-text="开启"
							inactive-text="关闭"
							inline-prompt
							style="width: 80px"
						/>
						<span style="padding-left: 10px; color: #ff6b6b">
							开启后，线索列表查询速度提升，但会占用更多内存</span
						>
					</el-form-item>

					<el-form-item label="启动同步">
						<el-switch
							v-model="basicForm.esSyncImmediately"
							:active-value="1"
							:inactive-value="0"
							active-text="开启"
							inactive-text="关闭"
							inline-prompt
							style="width: 80px"
						/>
						<span style="padding-left: 10px; color: #ff6b6b">
							开启后，将立即开始同步数据到搜索引擎，建议在业务低峰期开启</span
						>
					</el-form-item>

					<!-- esSyncStatus 显示同步状态，0 未同步 1 同步中 2 同步完成 3 同步失败 -->
					<el-form-item label="同步状态">
						<el-tag
							:type="
								basicForm.esSyncStatus === 0
									? 'info'
									: basicForm.esSyncStatus === 1
									? 'warning'
									: basicForm.esSyncStatus === 2
									? 'success'
									: basicForm.esSyncStatus === 3
									? 'danger'
									: 'info'
							"
							:icon="
								basicForm.esSyncStatus === 0
									? 'Close'
									: basicForm.esSyncStatus === 1
									? 'Clock'
									: basicForm.esSyncStatus === 2
									? 'Check'
									: basicForm.esSyncStatus === 3
									? 'Warning'
									: 'Close'
							"
						>
							{{
								basicForm.esSyncStatus === 0
									? "未同步"
									: basicForm.esSyncStatus === 1
									? "同步中..."
									: basicForm.esSyncStatus === 2
									? "同步完成"
									: basicForm.esSyncStatus === 3
									? "同步失败"
									: "未同步"
							}}
						</el-tag>
						<span style="padding-left: 10px; color: #ff6b6b">
							{{
								basicForm.esSyncStatus === 1
									? "ES数据正在同步中，一般同步需要10分钟左右，请稍候"
									: basicForm.esSyncStatus === 2
									? "ES数据同步已完成，搜索功能正常"
									: basicForm.esSyncStatus === 3
									? "ES数据同步失败，请检查网络连接或联系管理员"
									: "ES同步功能已开启"
							}}（过几秒，手动刷新页面看同步状态）
						</span>
					</el-form-item>
					<!-- esSyncStatus -->
				</el-tab-pane>

				<el-tab-pane label="项目配置">
					<el-form-item label="分配其他组员">
						<el-switch
							v-model="basicForm.projectAllGroup"
							:active-value="1"
							:inactive-value="0"
							active-text="可分配"
							inactive-text="关闭"
							inline-prompt
							style="width: 80px"
						/>
						<span style="padding-left: 10px; color: #ff6b6b">
							设置客服列表里面的成员是否可以分配线索给同项目内的其他组别成员
						</span>
					</el-form-item>
					<el-form-item label="多个项目管理员">
						<el-switch
							v-model="basicForm.projectMultiManger"
							:active-value="1"
							:inactive-value="0"
							active-text="多个"
							inactive-text="单个"
							inline-prompt
							style="width: 80px"
						/>
						<span style="padding-left: 10px; color: #ff6b6b">
							设置项目管理，每个项目是否可以添加多个项目管理员
						</span>
					</el-form-item>
					<el-form-item label="多个组主管">
						<el-switch
							v-model="basicForm.groupMultiManger"
							:active-value="1"
							:inactive-value="0"
							active-text="多个"
							inactive-text="单个"
							inline-prompt
							style="width: 80px"
						/>
						<span style="padding-left: 10px; color: #ff6b6b">
							设置项目管理，每个组别是否可以添加多个主管
						</span>
					</el-form-item>
				</el-tab-pane>

				<el-tab-pane label="分配配置">
					<div style="margin: 0 auto">
						<el-form-item label="分配方式">
							<el-select v-model="basicForm.allotType" style="width: 200px">
								<el-option label="不分配" :value="0" />
								<el-option label="按顺序分配" :value="1" />
								<el-option label="随机分配" :value="2" />
							</el-select>
						</el-form-item>

						<el-form-item label="推送开启">
							<el-switch
								v-model="basicForm.pushOpen"
								:active-value="1"
								:inactive-value="0"
								active-text="开启"
								inactive-text="关闭"
								inline-prompt
								style="width: 80px"
							/>
							<span style="padding-left: 10px; color: #ff6b6b">
								设置是否开启推送，把总接收栏目的数据推送到线索列表
							</span>
						</el-form-item>

						<el-tabs v-model="allotTab" style="margin-top: 20px">
							<el-tab-pane label="线索管理" name="public">
								<div class="public-allot">
									<el-form-item label="线索分配">
										<el-switch
											v-model="basicForm.allotOpen"
											:active-value="1"
											:inactive-value="0"
											active-text="开启"
											inactive-text="关闭"
											inline-prompt
											style="width: 80px"
										/>
										<span style="padding-left: 10px; color: #ff6b6b">
											设置是否开启线索分配
										</span>
									</el-form-item>

									<el-row :gutter="20">
										<el-col :span="12">
											<el-form-item label="当前分配人数">
												<el-input v-model="basicForm.sliceNum" disabled />
											</el-form-item>
										</el-col>
									</el-row>
									<el-form-item label="分配列表">
										<el-input
											v-model="basicForm.allotServicesNames"
											type="textarea"
											:rows="10"
											disabled
											style="width: 100%"
										/>
									</el-form-item>
									<el-form-item label="下次分配">
										<el-input v-model="basicForm.servicesNames" disabled />
									</el-form-item>
								</div>
							</el-tab-pane>
							<el-tab-pane label="线索地区" name="area">
								<div class="area-allot">
									<el-form-item label="地区分配">
										<el-switch
											v-model="basicForm.areaAllotOpen"
											:active-value="1"
											:inactive-value="0"
											active-text="开启"
											inactive-text="关闭"
											inline-prompt
											style="width: 80px"
										/>
										<span style="padding-left: 10px; color: #ff6b6b">
											设置是否开启地区分配
										</span>
									</el-form-item>
									<el-form-item label="区域名称">
										<el-input
											v-model="basicForm.areaName"
											type="textarea"
											:rows="4"
											:disabled="basicForm.areaAllotOpen === 0"
											style="width: 100%"
										/>
										<span style="padding-left: 10px; color: #ff6b6b">
											多个地区，每行一个区域名称，最多10个
										</span>
									</el-form-item>
									<el-row :gutter="20">
										<el-col :span="12">
											<el-form-item label="当前分配人数">
												<el-input
													v-model="basicForm.areaSliceNum"
													disabled
												/>
											</el-form-item>
										</el-col>
									</el-row>
									<el-form-item label="分配列表">
										<el-input
											v-model="basicForm.areaAllotServicesNames"
											type="textarea"
											:rows="10"
											disabled
											style="width: 100%"
										/>
									</el-form-item>
									<el-form-item label="下次分配">
										<el-input v-model="basicForm.areaServicesNames" disabled />
									</el-form-item>
								</div>
							</el-tab-pane>
							<el-tab-pane label="资源管理" name="resource">
								<div class="public-allot">
									<el-form-item label="资源分配">
										<el-switch
											v-model="basicForm.resourceAllotOpen"
											:active-value="1"
											:inactive-value="0"
											active-text="开启"
											inactive-text="关闭"
											inline-prompt
											style="width: 80px"
										/>
										<span style="padding-left: 10px; color: #ff6b6b">
											设置是否开启资源分配
										</span>
									</el-form-item>

									<el-row :gutter="20">
										<el-col :span="12">
											<el-form-item label="当前分配人数">
												<el-input
													v-model="basicForm.resourceSliceNum"
													disabled
												/>
											</el-form-item>
										</el-col>
									</el-row>
									<el-form-item label="分配列表">
										<el-input
											v-model="basicForm.resourceAllotServicesNames"
											type="textarea"
											:rows="10"
											disabled
											style="width: 100%"
										/>
									</el-form-item>
									<el-form-item label="下次分配">
										<el-input
											v-model="basicForm.resourceServicesNames"
											disabled
										/>
									</el-form-item>
								</div>
							</el-tab-pane>
						</el-tabs>
					</div>
				</el-tab-pane>
			</el-tabs>
		</el-form>
		<div style="display: flex; justify-content: flex-end; margin-top: 20px">
			<el-button type="primary" @click="submit">保存</el-button>
		</div>
	</div>
</template>
<script lang="ts" name="base-setting" setup>
import { useCool } from "/@/cool";
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";

// interface Contact {
// 	contactName: string;
// 	imgUrl: string;
// }

const { service } = useCool();
// const payType = ref([1, 2]);
// const basicForm: any = ref({}); // 表单数据
// const switchV = reactive({
// 	active: 1,
// 	inactiv: 0
// });

const basicForm = reactive({
	esOpen: 0,
	esSyncImmediately: 0,
	esSyncStatus: 0,
	projectAllGroup: 0,
	projectMultiManger: 0,
	groupMultiManger: 0,
	pushOpen: 0,
	allotType: 1,
	allotOpen: 0,
	sliceNum: 0,
	allotServicesNames: "",
	servicesNames: "",
	areaAllotOpen: 0,
	areaAllotType: 1,
	areaSliceNum: 0,
	areaAllotServicesNames: "",
	areaServicesNames: "",
	areaName: "",
	resourceAllotOpen: 0,
	resourceSliceNum: 0,
	resourceAllotServicesNames: "",
	resourceServicesNames: ""
});

const allotTab = ref("public");

const submit = async () => {
	// 互斥校验
	// if (basicForm.esSyncFullEnable === 1 && basicForm.esSyncIncrementEnable === 1) {
	// 	ElMessage.error("全量同步和增量同步不能同时开启！");
	// 	return;
	// }
	// 如果开启ES搜索，检查服务可用性
	// if (basicForm.esSearchOpen === 1) {
	// 	await checkESAvailable();
	// 	if (!available.value.available) {
	// 		ElMessage.error(available.value.message);
	// 		getBasic();
	// 		return;
	// 	}
	// }

	// 如果开启立即同步，但ES搜索未开启，提示用户
	// if (basicForm.esSyncFullEnable === 1 && basicForm.esOpen === 0) {
	// 	ElMessage.error("请先开启ES搜索功能，再开启数据同步");
	// 	return;
	// }

	// 校验区域名称数量
	let areaNames = (basicForm.areaName || "")
		.split("\n")
		.map((v) => v.replace(/\s+/g, "")) // 去除每行所有空格
		.filter((v) => v);
	if (areaNames.length > 10) {
		ElMessage.error("区域名称不能超过10个，每行一个区域名称！");
		return;
	}
	// 提交前去掉所有空格
	basicForm.areaName = areaNames.join("\n");

	service.customer_pro.basic
		.update({ ...basicForm })
		.then(() => {
			ElMessage.success("保存成功");
			getBasic();
		})
		.catch((err) => {
			ElMessage.error(err.message);
		});
};

// const available = ref<{ available: boolean; message: string }>({
// 	available: false,
// 	message: ""
// });
// const checkESAvailable = async () => {
// 	available.value = await service.customer_pro.clues.checkESAvailable();
// };

const getBasic = async () => {
	// 获取后端数据
	const data = await service.customer_pro.basic.info({ id: 1 });
	// 合并数据到 basicForm
	Object.assign(basicForm, data);
	// 计算当前分配人数
	if (basicForm.allotServicesNames) {
		basicForm.sliceNum = basicForm.allotServicesNames.split(",").filter(Boolean).length;
	} else {
		basicForm.sliceNum = 0;
	}
	if (basicForm.areaAllotServicesNames) {
		basicForm.areaSliceNum = basicForm.areaAllotServicesNames.split(",").filter(Boolean).length;
	} else {
		basicForm.areaSliceNum = 0;
	}
	if (basicForm.resourceAllotServicesNames) {
		basicForm.resourceSliceNum = basicForm.resourceAllotServicesNames
			.split(",")
			.filter(Boolean).length;
	} else {
		basicForm.resourceSliceNum = 0;
	}
};

onMounted(async () => {
	getBasic();
});
</script>
<style lang="scss" scoped>
.cl-form__footer {
	justify-content: flex-start !important;
}
</style>

<style lang="scss">
.view-my {
	background-color: var(--el-bg-color);
	// height: 100%;
	padding: 20px;
	box-sizing: border-box;

	.el-form {
		width: 100%;
		max-width: 100%;
	}

	.box-con {
		width: 100%;
		padding-bottom: 50px;

		.box-list {
			width: 100%;
			display: flex;
			flex-direction: row;
			flex-wrap: wrap;
			justify-content: flex-start;

			.box-item {
				width: 200px;
				margin-right: 20px;

				.box-col {
					display: flex;
					flex-direction: column;
					justify-content: center;
					align-items: center;
				}

				.avatar {
					width: 100%;
				}
			}
		}

		.add {
			width: 100%;
			text-align: center;
			font-size: 30px;
		}
	}

	.title {
		margin-bottom: 30px;
		font-size: 15px;
	}
}

.el-textarea__inner {
	min-height: 100px;
}

.avatar-uploader {
	margin-top: 20px;
	margin-bottom: 20px;
}

.avatar-uploader .el-upload {
	border: 1px dashed var(--el-border-color);
	border-radius: 6px;
	cursor: pointer;
	position: relative;
	overflow: hidden;
	transition: var(--el-transition-duration-fast);
}

.avatar-uploader .el-upload:hover {
	border-color: var(--el-color-primary);
}

.el-icon.avatar-uploader-icon {
	font-size: 28px;
	color: #8c939d;
	width: 178px;
	height: 178px;
	text-align: center;
}
</style>
