import { useCore, useTools } from "../../hooks";
import { computed, defineComponent, h, ref } from "vue";
interface SlotDomType {
	style: {
		color: string;
		fontWeight: string;
		float: string; // 注意：在CSS中，float属性已被弃用，你可能想使用其他属性如float的替代方案（如CSS Flexbox或Grid）
	};
	htmlText: string;
	selectNum: number;
}

export default defineComponent({
	name: "cl-pagination",
	props: {
		slotDom: {
			type: Object as () => SlotDomType | undefined, // 使用类型断言来指定Object的具体类型或undefined
			default: undefined // 提供一个默认值
		}
	},
	setup(props, { expose }) {
		const { crud, mitt } = useCore();
		const { style, browser } = useTools();

		// 自定义插槽
		const slotDom = computed(() => props.slotDom);

		// 总数
		const total = ref(0);

		// 当前页数
		const currentPage = ref(1);

		// 每页大小
		const pageSize = ref(20);

		// 页数发生变化
		function onCurrentChange(index: number) {
			crud.refresh({
				page: index
			});
		}

		// 条目发生变化
		function onSizeChange(size: number) {
			crud.refresh({
				page: 1,
				size
			});
		}

		function setPagination(res: DeepPartial<ClCrud.Pagination>) {
			if (res) {
				currentPage.value = res.currentPage || res.page || 1;
				pageSize.value = res.pageSize || res.size || 20;
				total.value = res.total || 0;
				crud.params.size = pageSize.value;
			}
		}

		mitt.on("crud.refresh", ({ pagination }: { pagination: ClCrud.Pagination }) => {
			setPagination(pagination);
		});

		expose({
			total,
			currentPage,
			pageSize,
			setPagination
		});

		return () => {
			return h(
				<el-pagination
					small={style.size == "small"}
					background
					page-sizes={[5, 10, 20, 30, 40, 50, 100]}
					layout={
						browser.isMini
							? "prev, pager, next"
							: "slot,total, sizes, prev, pager, next, jumper"
					}
				/>,
				{
					onSizeChange,
					onCurrentChange,
					total: total.value,
					currentPage: currentPage.value,
					pageSize: pageSize.value
				},

				() => {
					if (slotDom.value?.selectNum) {
						return h(
							"div",
							{ style: { ...slotDom.value?.style } },
							slotDom.value?.htmlText
						);
					}
				}
			);
		};
	}
});
