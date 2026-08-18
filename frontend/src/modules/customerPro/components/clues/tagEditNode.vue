<template>
	<div class="tag-node">
		<!-- 当前节点行 -->
		<div class="tag-node__row" :style="{ paddingLeft: depth * 24 + 'px' }">
			<!-- 缩进连接线 -->
			<div v-if="depth > 0" class="tag-node__indent-line" :style="{ left: (depth - 1) * 24 + 14 + 'px' }"></div>

			<!-- 展开/折叠按钮 -->
			<span
				class="tag-node__toggle"
				:class="{ 'has-children': item.children?.length }"
				@click="toggleCollapse"
			>
				<el-icon v-if="item.children?.length" :class="{ 'is-collapsed': item.collapsed }">
					<ArrowRight />
				</el-icon>
			</span>

			<!-- 名称输入 -->
			<el-input
				v-model="item.name"
				placeholder="标签名称"
				clearable
				size="default"
				class="tag-node__name"
			/>

			<!-- 添加子级按钮 -->
			<el-tooltip content="添加下级" placement="top">
				<el-button
					class="tag-node__action"
					link
					type="primary"
					@click="emit('addChild', item)"
				>
					<el-icon><Plus /></el-icon>
				</el-button>
			</el-tooltip>

			<!-- 展开/折叠高级配置 -->
			<el-icon
				class="tag-node__expand-btn"
				:class="{ 'is-expanded': item.expanded }"
				@click="item.expanded = !item.expanded"
			>
				<ArrowRight />
			</el-icon>

			<!-- 删除按钮 -->
			<el-button
				class="tag-node__action tag-node__action--danger"
				link
				type="danger"
				@click="emit('delete', item)"
			>
				<el-icon><Delete /></el-icon>
			</el-button>
		</div>

		<!-- 高级配置 -->
		<div v-show="item.expanded" class="tag-node__advanced" :style="{ marginLeft: depth * 24 + 32 + 'px' }">
			<div class="advanced-field">
				<span class="field-label">Value</span>
				<el-input v-model="item.value" placeholder="为空时自动生成" clearable size="small" />
			</div>
			<div class="advanced-field">
				<span class="field-label">排序</span>
				<el-input-number v-model="item.orderNum" :min="1" size="small" controls-position="right" />
			</div>
		</div>

		<!-- 子节点（递归） -->
		<div v-show="!item.collapsed" class="tag-node__children">
			<template v-if="item.children?.length">
				<tag-edit-node
					v-for="child in item.children"
					:key="child._uid"
					:item="child"
					:depth="depth + 1"
					@add-child="(p: any) => emit('addChild', p)"
					@delete="(i: any) => emit('delete', i)"
				/>
			</template>
		</div>
	</div>
</template>

<script lang="ts" setup>
import { ArrowRight, Plus, Delete } from "@element-plus/icons-vue";

interface TagEditItem {
	_uid: number;
	id?: any;
	name: string;
	value: string;
	orderNum: number;
	parentId: string;
	expanded: boolean;
	collapsed: boolean;
	children: TagEditItem[];
}

const props = defineProps<{
	item: TagEditItem;
	depth: number;
}>();

const emit = defineEmits<{
	(e: "addChild", parent: TagEditItem): void;
	(e: "delete", item: TagEditItem): void;
}>();

function toggleCollapse() {
	if (props.item.children?.length) {
		props.item.collapsed = !props.item.collapsed;
	}
}
</script>

<style lang="scss" scoped>
.tag-node {
	position: relative;

	&__row {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 3px 4px;
		border-radius: 6px;
		transition: background 0.18s;
		position: relative;

		&:hover {
			background: #f5f7fa;
		}
	}

	&__indent-line {
		position: absolute;
		top: 0;
		bottom: 0;
		width: 1px;
		background: #e4e7ed;
		pointer-events: none;

		&::after {
			content: "";
			position: absolute;
			bottom: 50%;
			left: 0;
			width: 10px;
			height: 1px;
			background: #e4e7ed;
		}
	}

	&__toggle {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		flex-shrink: 0;
		cursor: pointer;
		border-radius: 4px;
		transition: background 0.15s;

		.el-icon {
			font-size: 12px;
			color: #909399;
			transition: transform 0.2s ease, color 0.2s;

			&.is-collapsed {
				transform: rotate(0deg);
			}

			&:not(.is-collapsed) {
				transform: rotate(90deg);
				color: var(--color-primary);
			}
		}

		&:hover {
			background: #ecf5ff;
		}

		&:not(.has-children) {
			cursor: default;

			&:hover {
				background: transparent;
			}
		}
	}

	&__name {
		flex: 1;

		:deep(.el-input__wrapper) {
			border-radius: 6px;
		}
	}

	&__expand-btn {
		flex-shrink: 0;
		font-size: 14px;
		color: #c0c4cc;
		cursor: pointer;
		transition: transform 0.2s ease, color 0.2s;

		&:hover {
			color: var(--color-primary);
		}

		&.is-expanded {
			transform: rotate(90deg);
			color: var(--color-primary);
		}
	}

	&__action {
		flex-shrink: 0;
		padding: 4px;
		font-size: 14px;
		opacity: 0;
		transition: opacity 0.15s;

		.tag-node__row:hover & {
			opacity: 1;
		}

		&--danger {
			:deep(.el-icon) {
				font-size: 14px;
			}
		}
	}

	&__advanced {
		margin: 2px 0 4px;
		padding: 8px 12px;
		background: #f7f8fa;
		border-left: 3px solid var(--color-primary);
		border-radius: 0 6px 6px 0;

		.advanced-field {
			display: flex;
			align-items: center;
			gap: 8px;
			margin-bottom: 6px;

			&:last-child {
				margin-bottom: 0;
			}

			.field-label {
				display: inline-flex;
				align-items: center;
				flex-shrink: 0;
				width: 40px;
				height: 24px;
				font-size: 12px;
				color: #909399;
				justify-content: flex-end;
			}

			.el-input,
			.el-input-number {
				flex: 1;
			}

			.el-input-number {
				width: 120px;
			}
		}
	}

	&__children {
		position: relative;
	}
}
</style>
