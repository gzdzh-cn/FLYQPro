<template>
	<el-tabs v-model="activeName" type="border-card" @tab-click="handleClick" v-if="hasAnyPermission">
		<el-tab-pane label="编辑线索" name="edit" v-if="
			service.customer_pro[dtypeKey]._permission.update ||
			service.customer_pro.clues_filter._permission.update
		">
			<el-divider content-position="left">编辑线索</el-divider>
			<div class="sub-follow-container">
				<cl-form ref="FormEdit" :inner="true">
					<!-- 学校 -->
					<template #slot-schoolId="{ scope }">
						<el-select v-model="scope.schoolId" @change="schoolChange">
							<el-option v-for="item in schoolList" :key="item.value" :label="item.name"
								:value="item.id" />
						</el-select>
					</template>

					<!-- 专业 -->
					<template #slot-majorsId="{ scope }">
						<el-select v-model="scope.majorsId">
							<el-option v-for="item in majorsList" :key="item.value" :label="item.name"
								:value="item.id" />
						</el-select>
					</template>
				</cl-form>
			</div>
		</el-tab-pane>

		<el-tab-pane label="创建记录" name="order" v-if="hasFollowAddPermission && status == 0">
			<div style="min-height: 100px; height: 220px">
				<el-divider content-position="left">创建记录</el-divider>

				<cl-form ref="FormFollow" :inner="true">
					<template #slot-followType="{ scope }">
						<el-cascader v-model="scope.followType" :options="followOptions" @change="handleChange"
							:props="followProps" clearable placeholder="选择跟进方式" />
					</template>
					<template #slot-nextFollowupTime="{ scope }">
						<el-date-picker v-model="scope.nextFollowupTime" type="datetime" :default-time="defaultTime"
							value-format="YYYY-MM-DD HH:mm" time-format="HH:mm" :disabled-date="disabledDate"
							placeholder="选择跟进时间" />
					</template>
				</cl-form>
			</div>
		</el-tab-pane>

		<el-tab-pane label="跟进记录" name="follow" v-if="hasFollowListPermission && status == 0">
			<el-divider content-position="left">进度</el-divider>
			<div style="min-height: 100px; height: 400px; overflow: auto">
				<div style="min-height: 100px">
					<el-steps :active="flolowContent.length" direction="vertical" v-if="flolowContent">
						<el-step :icon="Edit" style="margin: 0 0 10px 0" v-for="(item, index) in flolowContent"
							:key="index">
							<template #title>
								<h4>{{ item.followTypeName }}</h4>
							</template>
							<template #description>
								<div style="">
									<p style="font-weight: 400">
										{{ item.remark }}
									</p>
									<p style="font-weight: 200">
										下次跟进时间：{{ item.nextFollowupTime }}
									</p>
									<p style="font-weight: 200">
										添加时间： {{ item.createTime }}「{{ item.userName }}」
									</p>
								</div>
							</template>
						</el-step>
					</el-steps>
				</div>
			</div>
		</el-tab-pane>

		<el-tab-pane label="聊天记录" name="talk" v-if="hasChatContentListPermission">
			<el-divider content-position="left">聊天记录</el-divider>
			<div v-if="!chatContentList || chatContentList.length == 0"
				style="text-align: center; padding: 40px; color: #999">
				暂无聊天记录
			</div>
			<el-tabs v-else tab-position="left" style="min-height: 100px; height: 400px">
				<el-tab-pane v-for="(chatContent, index) in chatContentList" :key="index">
					<template #label>
						<div class="legacy-chat-version-tab">
							<strong>版本{{ chatContent.chatContentVersion }}</strong>
							<span :title="versionKeyword(chatContent)">关键词：{{ versionKeyword(chatContent) }}</span>
						</div>
					</template>
					<div style="min-height: 100px; height: 500px; overflow: auto">
						<div class="legacy-chat-version-keyword">
							<span>关键词：</span><strong>{{ versionKeyword(chatContent) }}</strong>
						</div>
						<div v-if="!chatContent?.chat_content" style="text-align: center; padding: 40px; color: #999">
							该版本暂无聊天内容
						</div>
						<el-steps v-else :active="strToJson(chatContent?.chat_content).length" direction="vertical">
							<el-step :icon="strToJson(chatContent?.chat_content)[0]?.msg_type == 'g'
								? UserFilled
								: ChatLineRound
								" style="margin: 0 0 10px 0" v-for="(item, index) in strToJson(chatContent?.chat_content)" :key="index">
								<template #title>
									<h4>
										{{ item.msg_type == "g" ? "访客" : item.worker_name }}
									</h4>
								</template>
								<template #description>
									<div style="">
										<template v-if="item.msg?.includes('[IMG]')">
											<div v-for="(v, index) in extractImgTags(item.msg)" :key="index">
												<img :src="v" style="width: 180px" />
											</div>
										</template>
										<template v-else>
											<p style="font-weight: 400">
												<span v-html="item.msg"></span>
											</p>
										</template>

										<p style="font-weight: 200">
											{{
												dayjs(item.msg_time, "YYYYMMDDHHmmss").format(
													"YYYY-MM-DD HH:mm:ss"
												)
											}}
										</p>
									</div>
								</template>
							</el-step>
						</el-steps>
					</div>
				</el-tab-pane>
			</el-tabs>
		</el-tab-pane>
	</el-tabs>

	<div v-else style="text-align: center; padding: 40px; color: #999">暂无权限访问此功能</div>

	<div class="sumit-btn" v-if="activeName == 'order' || activeName == 'edit'">
		<el-button @click="cancel"> 取消 </el-button>
		<el-button type="success" @click="submitAll"> 保存 </el-button>
		<el-popconfirm v-if="cluesStatus == 0 && hasPushCommonClausePermission" title="确定放入公海吗?"
			@confirm="pushCommonClause">
			<template #reference>
				<el-button type="warning"> 放入公海 </el-button>
			</template>
		</el-popconfirm>
	</div>
</template>

<script lang="ts" name="customeer_pro-subFollow" setup>
import { useCool } from "/@/cool";
import { computed, onMounted, ref, watch } from "vue";
import { useForm } from "@cool-vue/crud";
import { dayjs, ElMessage } from "element-plus";
import { Edit, ChatLineRound, UserFilled } from "@element-plus/icons-vue";
import { useDict } from "/$/dict";

const props = defineProps({
	id: String,
	status: Number,
	dtype: {
		type: String,
		default: () => "clues"
	}
});

interface Chat {
	id?: string;
	cluesId?: string;
	guest_id?: string;
	chat_content: string;
	chatContentVersion?: number;
	keywords?: string;
}
interface FlolowContent {
	followTypeName?: string;
	nextFollowupTime?: string;
	createTime?: string;
	userName?: string;
	remark?: string;
}

const { dict } = useDict();
const emit = defineEmits(["cancel"]);
// 聊天记录
const chatContentList = ref<Chat[]>([]);
const versionKeyword = (chatContent: Chat) => chatContent.keywords?.trim() || "未记录";
const { service } = useCool();
const flolowContent = ref<FlolowContent[]>([]);
const cluesStatus = computed(() => props.status);

// 标签
const activeName = ref(props.status == 0 ? "order" : "edit");
const handleClick = () => { };

// 监听status变化，更新activeName
watch(
	() => props.status,
	(newStatus) => {
		activeName.value = newStatus == 0 ? "order" : "edit";
	}
);

const dtypeKey = computed(() => props.dtype as keyof typeof service.customer_pro);

const hasFollowAddPermission = computed(() => {
	return (service.customer_pro[dtypeKey.value] as any)?._permission?.followAdd || false;
});

const hasFollowListPermission = computed(() => {
	return (service.customer_pro[dtypeKey.value] as any)?._permission?.followList || false;
});

const hasChatContentListPermission = computed(() => {
	return (service.customer_pro[dtypeKey.value] as any)?._permission?.chatContentList || false;
});

const hasPushCommonClausePermission = computed(() => {
	return (service.customer_pro[dtypeKey.value] as any)?._permission?.pushCommonClause || false;
});

const hasAnyPermission = computed(() => {
	return (
		hasFollowAddPermission.value ||
		hasFollowListPermission.value ||
		hasChatContentListPermission.value ||
		(service.customer_pro[dtypeKey.value] as any)?._permission?.update ||
		service.customer_pro.clues_filter._permission.update
	);
});

const FormEdit = useForm();
async function openEdit() {
	const item = await service.customer_pro[dtypeKey.value].info({
		id: props.id
	});
	// console.log("从服务返回的原始 item:", item);
	// console.log("原始 item.level:", item.level);

	// 处理 level 字段
	if (item.level === '' || item.level === null || item.level === undefined) {
		// 空值设置为空数组
		item.level = [];
	} else if (typeof item.level === "string") {
		try {
			const parsed = JSON.parse(item.level);
			// 确保结果是数组，并且所有元素都是字符串
			if (Array.isArray(parsed)) {
				item.level = parsed.map(String).filter(Boolean);
			} else {
				item.level = [String(parsed)];
			}
		} catch {
			// 如果解析失败，将其作为单个字符串处理
			item.level = [item.level];
		}
	} else if (!Array.isArray(item.level)) {
		// 如果不是字符串也不是数组，包装成字符串数组
		item.level = [String(item.level)];
	} else {
		// 如果已经是数组，确保所有元素都是字符串，并过滤掉空值
		item.level = item.level.map(String).filter(Boolean);
	}

	// console.log("处理后的 level:", item.level);

	FormEdit.value?.open({
		title: "编辑线索",
		items: [
			() => {
				return {
					label: "53标识",
					prop: "guestId",
					span: 12,
					component: { name: "el-input", props: { disabled: true } }
				};
			},
			() => {
				return {
					label: "项目",
					prop: "projectId",
					span: 12,
					required: true,
					component: { name: "el-select", props: {} }
				};
			},
			() => {
				return {
					label: "IP",
					prop: "ip",
					span: 12,
					component: { name: "el-input", props: { disabled: true } }
				};
			},
			() => {
				return {
					label: "IP归属地",
					prop: "guestIpInfo",
					span: 12,
					component: { name: "el-input", props: { disabled: true } }
				};
			},
			{
				label: "姓名",
				prop: "name",
				span: 12,
				component: { name: "el-input" }
			},
			{
				label: "来源",
				prop: "sourceFrom",
				span: 12,
				required: true,
				component: { name: "el-select" }
			},
			{ label: "关键字", prop: "keywords", span: 12, component: { name: "el-input" } },
			{ label: "手机号", prop: "mobile", span: 12, component: { name: "el-input" } },
			{ label: "微信号", prop: "wechat", span: 12, component: { name: "el-input" } },
			{ label: "学历", prop: "education", span: 12, component: { name: "el-select" } },
			{
				label: "毕业院校",
				prop: "graduatedSchool",
				span: 12,
				component: { name: "el-input" }
			},

			{
				label: "意向院校",
				prop: "schoolId",
				span: 12,
				component: { name: "slot-schoolId" }
			},
			{
				label: "意向专业",
				prop: "majorsId",
				span: 12,
				component: { name: "slot-majorsId" }
			},

			{ label: "报读类型", prop: "majorsType", span: 12, component: { name: "el-select" } },
			{ label: "报读层次", prop: "degreeId", span: 12, component: { name: "el-select" } },

			{
				label: "户口类型",
				prop: "householdType",
				span: 12,
				component: { name: "el-select" }
			},
			{
				label: "户籍地址",
				prop: "householdAddress",
				span: 12,
				component: { name: "el-input" }
			},
			{
				label: "线索等级",
				prop: "level",
				span: 12,
				component: {
					name: "el-select",
					options: [],
					props: {
						multiple: true
					}
				}
			},
			{
				label: "性别",
				prop: "gender",
				span: 12,
				component: {
					name: "el-select"
				}
			},
			{
				label: "紧急联系人电话",
				prop: "emergencyMobile",
				props: {
					labelWidth: "130px"
				},
				span: 12,
				component: { name: "el-input" }
			},
			{
				label: "备注",
				prop: "remark",
				component: { name: "el-input", props: { type: "textarea", rows: 4 } }
			}
		],
		form: {
			...item
		},
		on: {
			async open() {
				getSchoolList();
				if (props.dtype == "resource") {
					FormEdit.value?.setProps("projectId", {
						disabled: true
					});
					FormEdit.value?.setProps("sourceFrom", {
						disabled: true
					});
				}

				// 项目
				const projectList = await service.customer_pro.project.list();
				FormEdit.value?.setOptions(
					"projectId",
					projectList
						.map((e) => {
							return {
								label: e.name,
								value: e.id
							};
						})
						.filter((item) => item.value != null)
				);

				// 报读类型
				const majorsTypeList = await service.customer_pro.readtypes.list();
				FormEdit.value?.setOptions(
					"majorsType",
					majorsTypeList
						.map((e) => {
							return {
								label: e.name,
								value: e.id
							};
						})
						.filter((item) => item.value != null)
				);

				// 报读层次
				const degreeList = await service.customer_pro.readdegree.list();
				FormEdit.value?.setOptions(
					"degreeId",
					degreeList
						.map((e) => {
							return {
								label: e.name,
								value: e.id
							};
						})
						.filter((item) => item.value != null)
				);

				// 户口性质
				FormEdit.value?.setOptions("householdType", [
					{
						label: "城镇",
						value: "1"
					},
					{
						label: "农村",
						value: "2"
					}
				]);

				// 线索级别
				const levelOptions = dict.get("cluesLevel");
				FormEdit.value?.setOptions(
					"level",
					levelOptions.value.filter((item) => item.value != null)
				);

				// 性别
				FormEdit.value?.setOptions("gender", [
					{
						label: "保密",
						value: "0"
					},
					{
						label: "男",
						value: "1"
					},
					{
						label: "女",
						value: "2"
					}
				]);

				// 来源
				FormEdit.value?.setOptions("sourceFrom", [
					{
						label: "手动录入",
						value: "1"
					},
					{
						label: "百度",
						value: "2"
					},
					{
						label: "抖音",
						value: "3"
					},
					{
						label: "53客服",
						value: "4"
					},
					{
						label: "小红书",
						value: "5"
					},
					{
						label: "线索资源",
						value: "6"
					}
				]);

				// 学历
				FormEdit.value?.setOptions("education", [
					{
						label: "未知",
						value: "1"
					},
					{
						label: "初中",
						value: "2"
					},
					{
						label: "高中/中专/中技",
						value: "3"
					},
					{
						label: "大专/高技",
						value: "4"
					},
					{
						label: "本科",
						value: "5"
					}
				]);
			},

			close(done) {
				done();
				emit("cancel");
			},
			submit(data, { close, done }) {
				// 处理 level 字段：空数组时发送 null
				if (Array.isArray(data.level) && data.level.length === 0) {
					data.level = null;
				}
				service.customer_pro[dtypeKey.value]
					.update({ ...data })
					.then(() => {
						done();
						close();
						ElMessage.success("保存成功");
					})
					.catch((e: any) => {
						console.log(e);
						ElMessage.error("保存失败", e.message);
					});
			}
		},
		op: {
			hidden: true
		}
	});
}

const FormFollow = useForm();
async function openFollow() {
	FormFollow.value?.open({
		title: "跟进",
		items: [
			() => {
				return {
					label: "跟进方式",
					prop: "followType",
					span: 12,
					component: { name: "slot-followType" }
				};
			},
			() => {
				return {
					label: "下次跟进时间",
					prop: "nextFollowupTime",
					span: 12,
					component: { name: "slot-nextFollowupTime" }
				};
			},
			{
				label: "备注",
				prop: "remark",
				component: { name: "el-input", props: { type: "textarea", rows: 4 } }
			}
		],
		on: {
			close(done) {
				done();
				emit("cancel");
			},
			submit(data, { close, done }) {
				if (data.followType) {
					if (data.nextFollowupTime == "" || data.nextFollowupTime == null) {
						ElMessage.error("请选择下次跟进时间");
						return;
					}
					if (data.remark == "" || data.remark == null) {
						ElMessage.error("请填写备注");
						return;
					}
					(service.customer_pro[dtypeKey.value] as any)
						.followAdd({ cluesId: props.id, ...data })
						.then(() => {
							done();
							close();
							// ElMessage.success("保存成功");
						})
						.catch((e: any) => {
							console.log(e);
							ElMessage.error("保存失败", e.message);
						});
				}
			}
		},
		op: {
			hidden: true
		}
	});
}

const submitAll = () => {
	const followType = FormFollow.value?.getForm("followType");
	const nextFollowupTime = FormFollow.value?.getForm("nextFollowupTime");
	const remark = FormFollow.value?.getForm("remark");
	if (followType && (nextFollowupTime == "" || nextFollowupTime == null)) {
		ElMessage.error("请选择下次跟进时间");
		return;
	}
	if (followType && (remark == "" || remark == null)) {
		ElMessage.error("请填写备注");
		return;
	}

	if (followType) {
		FormFollow.value?.submit();
		FormEdit.value?.submit();
	} else {
		FormEdit.value?.submit();
	}
};

const cancel = () => {
	FormEdit.value?.close();
	FormFollow.value?.close();
};

// 学校列表
const schoolList = ref();
const majorsList = ref();
const getSchoolList = async () => {
	schoolList.value = await service.customer_pro.school.list();
	schoolList.value[0]?.id && getMajorList(schoolList.value[0]?.id);
};
// 学校改变
const schoolChange = async (v: any) => {
	majorsList.value = [];
	FormEdit.value?.setForm("majorsId", null);
	getMajorList(v);
};

// 专业列表
const getMajorList = async (v: any) => {
	majorsList.value = await service.customer_pro.majors.list({ schoolId: v });
};

// 时间选择器起始
const defaultTime = new Date();
//禁用当前日期之前的日期
function disabledDate(time: { getTime: () => number }) {
	//Date.now()是javascript中的内置函数，它返回自1970年1月1日00:00:00 UTC以来经过的毫秒数。
	return time.getTime() < Date.now() - 8.64e7;
}

// 表格规则
// const rules = reactive({
// 	followType: [{ required: true, message: "选择跟进方式", trigger: "change" }],
// 	nextFollowupTime: [
// 		{
// 			required: true,
// 			message: "选择下次跟进时间",
// 			trigger: "change"
// 		}
// 	],
// 	remark: [
// 		{
// 			required: true,
// 			message: "填写备注",
// 			trigger: "change"
// 		}
// 	]
// });

// 跟进类型鼠标停留效果
const followProps = {
	expandTrigger: "hover" as const
};

// 跟进方式类型
const followOptions = ref([
	{
		label: "电话访谈",
		value: "2",
		children: [
			{
				label: "无人接听",
				value: "21"
			},
			{
				label: "拒接",
				value: "22"
			},
			{
				label: "已接通",
				value: "23"
			}
		]
	},
	{
		label: "微信沟通",
		value: "3",
		children: [
			{
				label: "待通过",
				value: "31"
			},
			{
				label: "拒绝通过",
				value: "32"
			},
			{
				label: "已通过",
				value: "33"
			}
		]
	},
	{
		label: "视频参观",
		value: "4"
	},
	{
		label: "预约参观",
		value: "5"
	},
	{
		label: "已参观",
		value: "6"
	}
]);

// 跟进方式改变
const handleChange = (e: any) => {
	console.log("handleChange", e);
};

// 放入公海
const pushCommonClause = () => {
	(service.customer_pro[dtypeKey.value] as any).pushCommonClause({ cluesId: props.id });
	ElMessage.success("保存成功");
	emit("cancel");
};

// 取图片地址
const extractImgTags = (content: any) => {
	const regex = /\[IMG\](.*?)\[\/IMG\]/g;
	let matches;
	const results = [];

	while ((matches = regex.exec(content)) !== null) {
		const strArr = matches[1].split("?");
		results.push(strArr[0]);
	}

	return results;
};

// 转json对象
const strToJson = (str: string) => {
	if (!str) {
		return [];
	}
	try {
		return JSON.parse(str);
	} catch (e) {
		console.warn("chatContent JSON解析失败:", e, str);
		return [];
	}
};

onMounted(async () => {
	openEdit();
	openFollow();
	// Crud.value?.refresh({ id: props.id });
	// 获取跟进记录
	if (hasFollowListPermission.value) {
		flolowContent.value = await (service.customer_pro[dtypeKey.value] as any).followList({
			cluesId: props.id
		});
	}

	// 获取53客服聊天记录
	if (hasChatContentListPermission.value) {
		try {
			const res = await (service.customer_pro[dtypeKey.value] as any).chatContentList({
				cluesId: props.id
			});
			chatContentList.value = Array.isArray(res) ? res : [];
		} catch (e) {
			console.error("获取聊天记录失败:", e);
			chatContentList.value = [];
		}
	}
});

defineExpose({
	pushCommonClause
});
</script>

<style>
.sumit-btn {
	display: flex;
	justify-content: right;
	padding-top: 20px;
}

.container {
	display: flex;
	flex-wrap: wrap;
	/* 允许子元素换行 */
	gap: 10px;
}

.item {
	flex-basis: calc(50% - 8px);
	/* 每行两个子元素，占据50%的宽度 */
	box-sizing: border-box;
	/* 包括内边距和边框在内 */
}

.sub-follow-container {
	height: 450px;
	overflow-y: auto;
	padding: 20px;
	box-sizing: border-box;
}

.legacy-chat-version-tab {
	display: flex;
	max-width: 170px;
	flex-direction: column;
	align-items: flex-start;
	line-height: 20px;
}

.legacy-chat-version-tab span {
	max-width: 100%;
	overflow: hidden;
	color: var(--el-text-color-secondary);
	font-size: 12px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.legacy-chat-version-keyword {
	margin: 0 16px 18px 0;
	padding: 10px 14px;
	border-radius: 6px;
	background: var(--el-fill-color-light);
	color: var(--el-text-color-regular);
}

.legacy-chat-version-keyword strong {
	color: var(--el-text-color-primary);
}
</style>
