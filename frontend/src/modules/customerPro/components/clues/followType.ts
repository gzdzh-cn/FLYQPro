export interface FollowOption {
	label: string;
	value: string;
	children?: FollowOption[];
}

// 跟进方式的唯一数据源：编辑下拉和跟进记录列表共用。
export const followOptions: FollowOption[] = [
	{
		label: "电话访谈",
		value: "2",
		children: [
			{ label: "无人接听", value: "21" },
			{ label: "拒接", value: "22" },
			{ label: "已接通", value: "23" }
		]
	},
	{
		label: "微信沟通",
		value: "3",
		children: [
			{ label: "待通过", value: "31" },
			{ label: "拒绝通过", value: "32" },
			{ label: "已通过", value: "33" }
		]
	},
	{ label: "视频参观", value: "4" },
	{ label: "预约参观", value: "5" },
	{ label: "已参观", value: "6" }
];

export const followProps = {
	expandTrigger: "hover" as const
};

const optionLabels = new Map<string, string>();
followOptions.forEach((option) => {
	optionLabels.set(option.value, option.label);
	option.children?.forEach((child) => optionLabels.set(child.value, child.label));
});

export function normalizeFollowType(value: unknown): string[] {
	if (value === null || value === undefined || value === "") return [];
	if (Array.isArray(value)) return value.map(String).filter(Boolean);

	let raw = String(value).trim();
	if (raw.startsWith("[") && raw.endsWith("]")) {
		try {
			const parsed = JSON.parse(raw);
			if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean);
		} catch {
			// 不是 JSON 数组时按普通字符串继续处理。
		}
	}

	const displayToValue: Record<string, string> = {
		"电话访谈 / 无人接听": "2,21",
		"电话访谈 / 拒接": "2,22",
		"电话访谈 / 已接通": "2,23",
		"微信沟通 / 待通过": "3,31",
		"微信沟通 / 拒绝通过": "3,32",
		"微信沟通 / 已通过": "3,33"
	};
	raw = displayToValue[raw] || raw;
	const values = raw.split(",").map((item) => item.trim()).filter(Boolean);
	if (values.length === 1 && ["21", "22", "23", "31", "32", "33"].includes(values[0])) {
		return [values[0].charAt(0), values[0]];
	}
	return values;
}

export function formatFollowType(value: unknown): string {
	const values = normalizeFollowType(value);
	if (values.length === 0) return "";
	return values.map((value) => optionLabels.get(value) || value).join(" / ");
}
