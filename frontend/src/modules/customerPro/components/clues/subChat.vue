<template>
	<div class="sub-chat">
		<div v-if="!chatContentList || chatContentList.length === 0" class="chat-empty">
			暂无聊天记录
		</div>
		<el-tabs v-else tab-position="left" class="chat-tabs">
			<el-tab-pane v-for="(chatContent, index) in chatContentList" :key="index">
				<template #label>
					<div class="chat-version-tab">
						<strong>版本{{ chatContent.chatContentVersion }}</strong>
						<span :title="versionKeyword(chatContent)">关键词：{{ versionKeyword(chatContent) }}</span>
					</div>
				</template>
				<div class="chat-scroll">
					<div class="chat-version-keyword">
						<span>关键词：</span>
						<strong>{{ versionKeyword(chatContent) }}</strong>
					</div>
					<div v-if="!chatContent?.chat_content" class="chat-empty">
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
								<div>
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
											dayjs(item.msg_time, "YYYYMMDDHHmmss").format("YYYY-MM-DD HH:mm:ss")
										}}
									</p>
								</div>
							</template>
						</el-step>
					</el-steps>
				</div>
			</el-tab-pane>
		</el-tabs>
	</div>
</template>

<script lang="ts" name="customer_pro-subChat" setup>
import { ref, onMounted, watch, computed } from "vue";
import { useCool } from "/@/cool";
import { dayjs } from "element-plus";
import { ChatLineRound, UserFilled } from "@element-plus/icons-vue";

interface ChatContent {
	id?: string;
	cluesId?: string;
	guest_id?: string;
	chat_content: string;
	chatContentVersion?: number;
	keywords?: string;
}

const props = defineProps<{
	cluesId?: string | number;
}>();

const { service } = useCool();
const chatContentList = ref<ChatContent[]>([]);

const versionKeyword = (chatContent: ChatContent) => chatContent.keywords?.trim() || "未记录";

const hasChatContentListPermission = computed(() => {
	return (service.customer_pro.clues as any)?._permission?.chatContentList || false;
});

const strToJson = (str: string) => {
	if (!str) return [];
	try {
		return JSON.parse(str);
	} catch (e) {
		console.warn("chatContent JSON解析失败:", e, str);
		return [];
	}
};

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

async function loadChatContent() {
	if (!props.cluesId || !hasChatContentListPermission.value) return;
	try {
		const res = await (service.customer_pro.clues as any).chatContentList({ cluesId: props.cluesId });
		chatContentList.value = Array.isArray(res) ? res : [];
	} catch (e) {
		console.error("获取聊天记录失败:", e);
		chatContentList.value = [];
	}
}

onMounted(() => {
	loadChatContent();
});

watch(() => props.cluesId, () => {
	loadChatContent();
});

defineExpose({ refresh: loadChatContent });
</script>

<style lang="scss" scoped>
.sub-chat {
	display: flex;
	flex-direction: column;
	height: 100%;
	padding: 0;

	.chat-empty {
		text-align: center;
		padding: 40px;
		color: #999;
	}

	.chat-tabs {
		flex: 1;
		min-height: 0;

		:deep(.el-tabs__content) {
			height: 100%;
		}

		:deep(.el-tab-pane) {
			height: 100%;
		}

		:deep(.el-tabs__item) {
			height: auto;
			min-height: 52px;
			padding-top: 7px;
			padding-bottom: 7px;
		}
	}

	.chat-version-tab {
		display: flex;
		max-width: 170px;
		flex-direction: column;
		align-items: flex-start;
		line-height: 20px;

		span {
			max-width: 100%;
			overflow: hidden;
			color: var(--el-text-color-secondary);
			font-size: 12px;
			text-overflow: ellipsis;
			white-space: nowrap;
		}
	}

	.chat-version-keyword {
		margin-bottom: 18px;
		padding: 10px 14px;
		border-radius: 6px;
		background: var(--el-fill-color-light);
		color: var(--el-text-color-regular);

		strong {
			color: var(--el-text-color-primary);
		}
	}

	.chat-scroll {
		height: 100%;
		overflow-y: auto;
		padding: 0 16px 16px;

		:deep(.el-step__description) {
			color: #333;
			p {
				color: #333;
			}
		}

		:deep(.el-step__title) {
			color: #333;
		}
	}
}
</style>
