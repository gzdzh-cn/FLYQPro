/**
 * 字典数据工具函数
 */

/**
 * 为字典选项添加type字段
 * @param dictOptions 字典选项数组
 * @param typeMapping 值到类型的映射关系
 * @returns 添加了type字段的选项数组
 */
export function addTypeToDictOptions(
	dictOptions: Array<{ label: string; value: any }>,
	typeMapping: Record<any, string>
) {
	return dictOptions.map((option) => ({
		...option,
		type: typeMapping[option.value] || "default"
	}));
}

/**
 * 为字典选项添加自定义字段
 * @param dictOptions 字典选项数组
 * @param fieldMapping 字段映射函数
 * @returns 添加了自定义字段的选项数组
 */
export function addCustomFieldsToDictOptions<T extends Record<string, any>>(
	dictOptions: Array<{ label: string; value: any }>,
	fieldMapping: (option: { label: string; value: any }) => T
) {
	return dictOptions.map((option) => ({
		...option,
		...fieldMapping(option)
	}));
}

/**
 * 常用的状态类型映射
 */
export const STATUS_TYPE_MAPPING = {
	// 开启/关闭状态
	ENABLE_DISABLE: {
		1: "success", // 开启
		0: "danger" // 关闭
	},
	// 是/否状态
	YES_NO: {
		1: "success", // 是
		0: "info" // 否
	},
	// 审核状态
	AUDIT_STATUS: {
		0: "info", // 待审核
		1: "success", // 通过
		2: "danger" // 拒绝
	},
	// 收入
	INCOME_TYPE: {
		out: "success", // 收入
		in: "danger" // 支出
	}
} as const;
