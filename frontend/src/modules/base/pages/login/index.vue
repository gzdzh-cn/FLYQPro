<template>
	<div class="select-none">
		<img src="/static/img/bg.png" class="wave" />

		<div class="login-container">
			<div class="img">
				<LoginLeft />
			</div>

			<div class="login-box">
				<div class="login-form">
					<img class="avatar" :src="logo" alt="Logo" @error="onLogoError" />

					<h2 class="outline-none">
						<div>
							<span class="type-it">
								{{ siteName }}
							</span>
						</div>
					</h2>
					<el-form ref="ruleFormRef" size="large">
						<!-- 账号登录 -->
						<div v-show="loginM == 'account'">
							<el-form-item prop="username">
								<el-input v-model="form.username" clearable placeholder="请输入用户名" maxlength="20"
									autocomplete="on" :prefix-icon="User" />
							</el-form-item>

							<el-form-item prop="password">
								<el-input v-model="form.password" clearable show-password placeholder="请输入密码"
									:prefix-icon="Lock" />
							</el-form-item>

							<el-form-item prop="verifyCode">
								<el-input v-model="form.verifyCode" clearable placeholder="请输入验证码" :prefix-icon="Key"
									@keyup.enter="toLogin">
									<template v-slot:append>
										<captcha :ref="setRefs('captcha')" v-model="form.captchaId" @change="
											() => {
												form.verifyCode = '';
											}
										" />
									</template>
								</el-input>
							</el-form-item>
						</div>

						<!-- 手机号登录 -->
						<div v-show="loginM == 'mobile'">
							<el-form ref="ruleFormRef" size="large">
								<el-form-item prop="phone">
									<el-input v-model="form.phone" class="code" placeholder="请输入手机号"
										:prefix-icon="Iphone">
										<template #suffix>
											<el-button @click="getCode" style="border: none">
												<span v-show="show">发送验证码</span>
												<span v-show="!show" class="count">{{ count }} s</span>
											</el-button>
										</template>
									</el-input>
								</el-form-item>
								<el-form-item label="验证码">
									<el-input v-model="form.code" type="text" placeholder="请输入手机验证码" :prefix-icon="Key"
										maxlength="4" autocomplete="off" @keyup.enter="toLogin" />
								</el-form-item>
							</el-form>
						</div>

						<el-form-item>
							<el-button class="w-full mt-4" size="default" type="primary" :loading="saving"
								@click="toLogin" :disabled="disabled">
								登录
							</el-button>
						</el-form-item>

						<el-form-item>
							<div class="mutiType">
								<el-button v-for="(item, index) in operates" :key="index"
									:class="loginM == item.type ? 'active' : ''" @click="loginChange(item.type)">
									{{ item.title }}
								</el-button>
							</div>
						</el-form-item>
					</el-form>
				</div>
			</div>
		</div>
		<div class="bottom-text" v-html="copyright"></div>
	</div>
</template>

<script lang="ts" name="login" setup>
import { onMounted, reactive, ref, toRaw } from "vue";
import { ElMessage } from "element-plus";
import { config, useCool } from "/@/cool";
import { useBase } from "/$/base";
import Captcha from "./components/captcha.vue";
import { User, Lock, Key, Iphone } from "@element-plus/icons-vue";
import LoginLeft from "./svg/login-left.vue";
import { watch } from "vue";
import { request } from "/@/cool/service/request";

const { refs, setRefs, router, service } = useCool();
const { user, app } = useBase();

const disabled = ref(false);
const defaultLogo = "/logo.png";
const logo = ref(app.info.logo || defaultLogo);
const siteName = ref(app.info.name);
const copyright = ref(app.info.copyright);

watch(
	() => app.info,
	(newInfo) => {
		logo.value = newInfo.logo || defaultLogo;
		siteName.value = newInfo.name;
		copyright.value = newInfo.copyright;
	},
	{ deep: true }
);

function onLogoError() {
	if (logo.value !== defaultLogo) {
		logo.value = defaultLogo;
	}
}

onMounted(() => {
	// 初始化时设置值
	// logo.value = setting.setting.logo || app.info.logo;
	// siteName.value = setting.setting.siteName || app.info.name;
});
const operates = [
	{
		title: "账号登录",
		type: "account"
	}
	// {
	// 	title: "短信登录",
	// 	type: "mobile"
	// }
];

// 切换模式
const loginM = ref("account");
const loginChange = (model: string) => {
	loginM.value = model;
};

const show = ref(true);
const count = ref(0);

// 获取验证码
const getCode = async () => {
	// if (!form.phone) {
	// 	return ElMessage.error("手机号码不能为空");
	// }
	// try {
	// 	await service.base.open.sendSms({ phone: form.phone }).then((res) => {
	// 		show.value = false;
	// 		count.value = 100;
	// 		timeLoop();
	// 		return ElMessage.success("短信已发送");
	// 	});
	// } catch (err: any) {
	// 	return ElMessage.error(err.message);
	// }
};

const timeLoop = () => {
	const interval = setInterval(() => {
		count.value--;
		if (count.value < 0) {
			clearInterval(interval);
			show.value = true;
		}
	}, 1000);
};
// 状态1
const saving = ref(false);

// 表单数据
const form = reactive({
	username: "",
	password: "",
	captchaId: "",
	verifyCode: "",
	phone: "",
	code: ""
});

const formMobile = reactive({
	phone: "",
	code: ""
});

// 登录
async function toLogin() {
	if (loginM.value == "account") {
		if (!form.username) {
			return ElMessage.error("用户名不能为空");
		}

		if (!form.password) {
			return ElMessage.error("密码不能为空");
		}

		if (!form.verifyCode) {
			return ElMessage.error("图片验证码不能为空");
		}
	} else {
		if (!form.phone) {
			return ElMessage.error("手机号码不能为空");
		}

		if (!form.code) {
			return ElMessage.error("验证码不能为空");
		}
	}

	saving.value = true;

	try {
		let res: any;
		// 登录
		if (loginM.value == "account") {
			// 显式提交 JSON，避免 Wails WebView 下动态 EPS 方法丢失请求体。
			res = await request({
				// Wails 的 /dev 资源代理转发 POST 时可能丢失请求体，登录直接访问 GoFrame 服务。
				url: `${config.host}/admin/base/open/login`,
				method: "POST",
				data: { ...toRaw(form) }
			});
		} else {
			// res = await service.base.open.smsLogin(form)
		}
		user.setToken(res);

		// token 事件
		await Promise.all(app.events.hasToken.map((e) => e()));

		// 跳转
		router.push("/");
	} catch (err: any) {
		if (loginM.value == "account") {
			refs.captcha.refresh();
		}
		ElMessage.error(err.message);
	}
	saving.value = false;
}
</script>

<style lang="scss" scoped>
.code :deep(.el-input__wrapper) {
	padding-right: 1px;
}
</style>

<style scoped>
@import url("/static/style/login.css");
</style>

<style lang="scss" scoped>
:deep(.el-input-group__append, .el-input-group__prepend) {
	padding: 0;
}

.translation {
	:deep(.el-dropdown-menu__item) {
		padding: 5px 40px;
	}

	.check-zh {
		position: absolute;
		left: 20px;
	}

	.check-en {
		position: absolute;
		left: 20px;
	}
}

.w-full {
	width: 100%;
}

.form-box {
	width: 80%;

	.logo {
		width: 100%;
	}
}

.mutiType {
	width: 100%;
	display: flex;
	justify-content: space-around;
	align-items: center;
	gap: 10px;
}

.bottom-text {
	width: 100%;
	font-size: 12px;
	text-align: center;
	color: #999;
	position: absolute;
	bottom: 20px;

	:deep a {
		text-align: center;
		color: #999 !important;
	}
}

.active {
	background-color: var(--color-primary);
	color: #fff;
}
</style>
