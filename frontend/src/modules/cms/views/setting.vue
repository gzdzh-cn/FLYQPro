<script lang="ts" name="common-setting" setup>
import { useCool } from "/@/cool";
import { onMounted, reactive, ref, watch } from "vue";
import { ElMessage, ElLoading } from "element-plus";
import { Plus, Minus, Upload } from "@element-plus/icons-vue";

const { service } = useCool();
const payType = ref([1, 2]);
// 获取资料
const getInfo = async () => {
	basicForm.value = await service.cms.setting.info({ id: 1 });
	contactArr.value = basicForm.value.contactList
		? JSON.parse(basicForm.value.contactList)
		: contactArr.value;
};
// 保存状态
const loading = ref(false);
const loadingPage = ref();

interface Contact {
	contactName: string;
	imgUrl: string;
}

const contactArr = ref<Contact[]>([{ contactName: "", imgUrl: "" }]);

const add = () => {
	contactArr.value.push({ contactName: "", imgUrl: "" });
};

const del = (index: number) => {
	if (contactArr.value.length == 1) {
		ElMessage.warning("至少保留一个");
	} else {
		contactArr.value.splice(index, 1);
	}
};

const success = (item: any) => {
	contactArr.value[contactArr.value.length - 1].imgUrl = item.url;
	loading.value = false;
	loadingPage.value.close();
};

const currentIndex = ref(0);
const getIndex = (index: number) => {
	loadingPage.value = ElLoading.service({
		lock: true,
		text: "上传中",
		background: "rgba(0, 0, 0, 0.7)"
	});
	loading.value = true;
	currentIndex.value = index;
};

watch(
	() => contactArr.value,
	(n) => {
		console.log("contactArr", contactArr.value);
	},
	{
		immediate: true,
		deep: true
	}
);

// 表单数据
const basicForm: any = ref({
	sitename: "",
	notic: "",
	image: "",
	contact: "",
	mobile: "",
	gpt4: 0,
	contactList: []
	// domainName: "",
	// logo: "",
	// company: "",
	// contact: "",
	// contactWay: "",
	// mobile: "",
	// address: "",
	// keyword: "",
	// description: "",
	// fieldJson: "",

	// remindDay: 0,
	// remindEmail: 0,
	// remindSms: 0,
	// sendEmail: "",
	// requestEmail: "",
	// smtp: "",
	// pass: "", //邮箱授权码

	// accessKeyId: "",
	// accessKeySecret: "",
	// signName: "",
	// templateCode: "",
	// endpoint: "",
	// remindMobile: ""
});

const switchV = reactive({
	active: 1,
	inactiv: 0
});

// 保存
const save = () => {
	loading.value = false;
	basicForm.value.contactList = contactArr.value;
	service.cms.setting
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
				<el-form class="card-form" label-width="160px" :model="basicForm" :disabled="loading">
					<el-form-item label="站点名称">
						<el-input v-model="basicForm.sitename" placeholder="请填写站点名称" clearable />
					</el-form-item>



					<el-form-item label="明信片">
						<cl-upload v-model="basicForm.image" text="明信片" type="image" />
					</el-form-item>

					<el-form-item label="联系人">
						<el-input v-model="basicForm.contact" placeholder="请填写联系人" clearable />
					</el-form-item>

					<el-form-item label="手机号码">
						<el-input v-model="basicForm.mobile" placeholder="请填写手机号码" clearable />
					</el-form-item>

				</el-form>
			</el-tab-pane>

			<el-tab-pane label="支付">
				<el-form class="card-form" label-width="160px" :model="basicForm" :disabled="loading">
					<el-form-item label="模式">
						<el-radio-group v-model="basicForm.payType" class="ml-4">
							<el-radio :label="payType[0]" size="large"> 普通商户模式 </el-radio>
							<el-radio :label="payType[1]" size="large"> 服务商模式 </el-radio>
						</el-radio-group>
					</el-form-item>

					<!-- 普通商户模式 -->
					<div v-show="basicForm.payType == 1">
						<el-form-item label="普通商户号">
							<el-input v-model="basicForm.mchId" placeholder="请填写普通商户号" clearable />
						</el-form-item>
						<el-form-item label="普通商户appid">
							<el-input v-model="basicForm.appid" placeholder="请填写普通商户appid" clearable />
						</el-form-item>
						<el-form-item label="收款商户v3密钥">
							<el-input v-model="basicForm.cAPIv3Key" placeholder="请填写收款商户v3密钥" clearable />
						</el-form-item>
						<el-form-item label="序列号">
							<el-input v-model="basicForm.cSerialNo" placeholder="请填写序列号" clearable />
						</el-form-item>
					</div>
					<!-- 普通商户模式 -->

					<!-- 服务商模式 -->
					<div v-show="basicForm.payType == 2">
						<el-form-item label="服务商商户号">
							<el-input v-model="basicForm.spMchid" placeholder="请填写服务商商户号" clearable />
						</el-form-item>
						<el-form-item label="服务商appid">
							<el-input v-model="basicForm.spAppid" placeholder="请填写服务商appid" clearable />
						</el-form-item>
						<el-form-item label="特约商户">
							<el-input v-model="basicForm.subMchId" placeholder="请填写特约商户" clearable />
						</el-form-item>
						<el-form-item label="收款商户v3密钥">
							<el-input v-model="basicForm.aPIv3Key" placeholder="请填写收款商户v3密钥" clearable />
						</el-form-item>
						<el-form-item label="序列号">
							<el-input v-model="basicForm.serialNo" placeholder="请填写序列号" clearable />
						</el-form-item>
					</div>
					<!-- 服务商模式 -->

					<el-form-item label="支付回调地址">
						<el-input v-model="basicForm.notifyUrl" placeholder="请填写支付回调地址" clearable />
					</el-form-item>
				</el-form>
			</el-tab-pane>

			<el-tab-pane label="翻译">
				<el-form class="card-form" label-width="160px" :model="basicForm" :disabled="loading">
					<el-form-item label="百度apiKey">
						<el-input v-model="basicForm.baiduTranApiKey" placeholder="请填写百度apiKey" clearable />
					</el-form-item>

					<el-form-item label="百度secretKey">
						<el-input v-model="basicForm.baiduTranSecretKey" placeholder="请填写百度secretKey" clearable />
					</el-form-item>
				</el-form>
			</el-tab-pane>

			<el-tab-pane label="客服">
				<div class="box-con">
					<div class="box-list">
						<div class="box-item" v-for="(item, index) in contactArr" :key="index">
							<el-card style="margin-bottom: 30px">
								<div class="box-col">
									<el-input v-model="item.contactName" placeholder="请填写客服名称" clearable />

									<div style="padding: 10px 0 10px 0">
										<cl-upload text="选择图片" v-model="item.imgUrl" @success="success">
											<el-button :icon="Upload" @click="getIndex(index)">上传</el-button>
										</cl-upload>
									</div>

									<div class="del">
										<el-button type="danger" :icon="Minus" @click="del(index)">删除</el-button>
									</div>
								</div>
							</el-card>
						</div>
					</div>

					<div class="add">
						<el-button type="info" :icon="Plus" @click="add">增加一个</el-button>
					</div>
				</div>
			</el-tab-pane>

			<el-form-item>
				<el-button type="primary" :disabled="loading" @click="save">保存修改</el-button>
			</el-form-item>

			<!-- <el-tab-pane label="配置">
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
			</el-tab-pane> -->
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
