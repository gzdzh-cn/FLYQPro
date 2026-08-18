// 导出字典工具函数
export * from "./dict-helper";

//过滤所有的html标签
export const filterHtml = (str: any) => {
	var reg = /<[^>]+>/gi;
	return str && str.replace(reg, "").replace("\n\r", "");
};

/*去掉html标签（真正意义上去掉所有html标签包括内嵌的css样式）*/
export const stripHTML = (html: any, isRemoveNewLine: boolean) => {
	let t = document.createElement("div");
	t.innerHTML = html;
	let r = t.innerText;
	isRemoveNewLine && (r = r.replace(/[\r\n]/g, ""));
	return r;
};

// 截取string指定数量
export const subString = (str: string, num?: number) => {
	if (!num) {
		num = 100;
	}
	if (!str) {
		return "";
	}
	if (str.length <= num) {
		return str; // 字符串长度不超过100，无需截取
	} else {
		return str.slice(0, num) + "..."; // 截取前100个字符，并添加省略号
	}
};

// 日期转时间戳
export const datetimeToTimestamp = (dateTime: string | number | Date) => {
	var date = new Date(dateTime);
	var timestamp = date.getTime();
	return timestamp;
};

/**
 * 判断时间
 * @param date1
 * @param date2
 */
export const compareDate = (date1: string, date2: string) => {
	const t1 = datetimeToTimestamp(date1);
	const t2 = datetimeToTimestamp(date2);
	if (t1 > t2) {
		return true;
	} else {
		return false;
	}
};

// 遍历树
export const getTreeData = (arr: any[], parentId: any) => {
	function loop(parentId: any) {
		return arr.reduce((pre, cur) => {
			if (cur.parentId === parentId) {
				cur.children = loop(cur.id);
				pre.push(cur);
			}

			return pre;
		}, []);
	}

	return loop(parentId);
};

// 遍历树
export const deepTree = (arr: any[]) => {
	// 利用两层filter实现
	return arr.filter((item) => {
		item.children = arr.filter((e) => {
			return item.id === e.parentId;
		});
		return !item.parentId;
	});
};
