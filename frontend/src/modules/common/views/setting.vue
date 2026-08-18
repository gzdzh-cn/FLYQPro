<script lang="ts" name="portal-setting" setup>
import { useCool } from "/@/cool";
import { onMounted, reactive, ref } from "vue";
import { ElMessage } from "element-plus";
const { service } = useCool();

// 获取资料
const getInfo = async () => {
	basicForm.value = await service.common.setting.info({ id: 1 });
};
// 保存状态
const loading = ref(false);

// 表单数据
const basicForm: any = ref({
	siteName: "",
	domainName: "",
	logo: "",
	company: "",
	contact: "",
	contactWay: "",
	mobile: "",
	address: "",
	keyword: "",
	description: "",
	fieldJson: "",

	remindDay: 0,
	remindEmail: 0,
	remindSms: 0,
	sendEmail: "",
	requestEmail: "",
	smtp: "",
	pass: "", //邮箱授权码

	accessKeyId: "",
	accessKeySecret: "",
	signName: "",
	templateCode: "",
	endpoint: "",
	remindMobile: ""
});

const switchV = reactive({
	active: 1,
	inactiv: 0
});

// 保存
const save = () => {
	loading.value = false;
	service.common.setting
		.update({
			id: 1,
			...basicForm.value
		})
		.then(() => {
			ElMessage.success("更新成功");
		});
};

onMounted(() => {
	getInfo();
});
</script>
<template>
	<div class="view-my">
		<el-tabs type="border-card">
			<el-tab-pane label="基本信息">
				<el-form label-width="100px" :model="basicForm" :disabled="loading">
					<el-form-item label="网站名称">
						<el-input
							v-model="basicForm.siteName"
							placeholder="请填写网站名称"
							clearable
						/>
					</el-form-item>

					<el-form-item label="网站域名">
						<el-input
							v-model="basicForm.domainName"
							placeholder="请填写网站域名"
							clearable
						/>
					</el-form-item>

					<el-form-item label="logo">
						<cl-upload v-model="basicForm.logo" />
					</el-form-item>

					<el-form-item label="公司名称">
						<el-input
							v-model="basicForm.company"
							placeholder="请填写公司名称"
							clearable
						/>
					</el-form-item>

					<el-form-item label="联系人">
						<el-input
							v-model="basicForm.contact"
							placeholder="请填写联系人"
							clearable
						/>
					</el-form-item>

					<el-form-item label="座机">
						<el-input
							v-model="basicForm.contactWay"
							placeholder="请填写座机"
							clearable
						/>
					</el-form-item>

					<el-form-item label="手机">
						<el-input v-model="basicForm.mobile" placeholder="请填写手机" clearable />
					</el-form-item>

					<el-form-item label="地址">
						<el-input v-model="basicForm.address" placeholder="请填写地址" clearable />
					</el-form-item>

					<el-form-item label="关键词">
						<el-input
							v-model="basicForm.keyword"
							placeholder="请填写关键词"
							maxlength="30"
							show-word-limit
							type="textarea"
							:rows="6"
						/>
					</el-form-item>

					<el-form-item label="描述">
						<el-input
							v-model="basicForm.description"
							placeholder="请填写描述"
							maxlength="80"
							show-word-limit
							type="textarea"
							:rows="6"
						/>
					</el-form-item>

					<el-form-item>
						<el-button type="primary" :disabled="loading" @click="save"
							>保存修改</el-button
						>
					</el-form-item>
				</el-form>
			</el-tab-pane>
			<el-tab-pane label="配置">
				<el-form label-width="100px" :model="basicForm" :disabled="loading">
					<el-tabs model-value="basicConfig">
						<el-tab-pane label="通知设置" name="basicConfig">
							<el-form-item label="到期天数提醒">
								<el-input-number
									v-model="basicForm.remindDay"
									placeholder="提前天数"
									:min="1"
									:max="100"
									controls-position="right"
								/>
							</el-form-item>
							<el-form-item label="到期邮件提醒">
								<el-switch
									v-model="basicForm.remindEmail"
									:active-value="switchV.active"
									:inactiv-value="switchV.inactiv"
								/>
							</el-form-item>
							<el-form-item label="到期短信提醒">
								<el-switch
									v-model="basicForm.remindSms"
									:active-value="switchV.active"
									:inactiv-value="switchV.inactiv"
								/>
							</el-form-item>
						</el-tab-pane>
						<el-tab-pane label="邮件设置" name="emailConfig">
							<el-form-item label="发送邮箱">
								<el-input
									v-model="basicForm.sendEmail"
									placeholder="请填写发送邮箱"
									clearable
								/>
							</el-form-item>
							<el-form-item label="smtp">
								<el-input
									v-model="basicForm.smtp"
									placeholder="请填写smtp"
									clearable
								/>
							</el-form-item>

							<el-form-item label="邮箱授权码">
								<el-input
									v-model="basicForm.pass"
									placeholder="请填写邮箱授权码"
									type="password"
									clearable
								/>
							</el-form-item>

							<el-form-item label="接收邮箱">
								<el-input
									v-model="basicForm.requestEmail"
									placeholder="请填写接收邮箱"
									clearable
								/>
							</el-form-item>
						</el-tab-pane>
						<el-tab-pane label="短信设置" name="second">
							<el-form-item label="accessKeyId">
								<el-input
									v-model="basicForm.accessKeyId"
									placeholder="请填写accessKeyId"
									clearable
								/>
							</el-form-item>
							<el-form-item label="accessKeySecret">
								<el-input
									v-model="basicForm.accessKeySecret"
									placeholder="请填写accessKeySecret"
									clearable
								/>
							</el-form-item>

							<el-form-item label="签名">
								<el-input
									v-model="basicForm.signName"
									placeholder="请填写签名"
									clearable
								/>
							</el-form-item>

							<el-form-item label="模板">
								<el-input
									v-model="basicForm.templateCode"
									placeholder="请填写模板"
									clearable
								/>
							</el-form-item>

							<el-form-item label="endpoint">
								<el-input
									v-model="basicForm.endpoint"
									placeholder="请填写endpoint"
									clearable
								/>
							</el-form-item>
							<el-form-item label="通知手机">
								<el-input
									v-model="basicForm.remindMobile"
									placeholder="请填写通知手机"
									clearable
								/>
							</el-form-item>
						</el-tab-pane>
					</el-tabs>

					<el-form-item>
						<el-button type="primary" :disabled="loading" @click="save"
							>保存修改</el-button
						>
					</el-form-item>
				</el-form>
			</el-tab-pane>
		</el-tabs>
	</div>
</template>

<style lang="scss">
.view-my {
	background-color: var(--el-bg-color);
	// height: 100%;
	padding: 20px;
	box-sizing: border-box;

	.el-form {
		width: 600px;
		max-width: 100%;
	}

	.title {
		margin-bottom: 30px;
		font-size: 15px;
	}
}
.el-textarea__inner {
	min-height: 100px;
}
</style>
