# Wails3 本地图片读取指南

在 Wails3 中读取本地绝对路径的图片，需要通过 Go 后端服务来实现。本指南将详细介绍实现方法。

## 1. 后端实现 (Go)

### 1.1 在 GreetService 中添加方法

在 `greetservice.go` 中已经实现了 `GetLocalImage` 方法：

```go
// GetLocalImage 读取本地图片并返回 base64 编码
func (gs *GreetService) GetLocalImage(filePath string) (string, error) {
	// 检查文件是否存在
	if !gfile.Exists(filePath) {
		return "", errors.New("文件不存在: " + filePath)
	}

	// 读取文件内容
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 获取文件扩展名来确定 MIME 类型
	ext := gfile.Ext(filePath)
	var mimeType string
	switch ext {
	case ".jpg", ".jpeg":
		mimeType = "image/jpeg"
	case ".png":
		mimeType = "image/png"
	case ".gif":
		mimeType = "image/gif"
	case ".webp":
		mimeType = "image/webp"
	default:
		mimeType = "image/jpeg" // 默认
	}

	// 将文件内容转换为 base64
	base64Data := base64.StdEncoding.EncodeToString(fileContent)

	// 返回 data URL 格式
	return "data:" + mimeType + ";base64," + base64Data, nil
}
```

### 1.2 方法说明

- **参数**: `filePath` - 本地图片的绝对路径
- **返回值**: base64 编码的 data URL 格式字符串
- **功能**: 
  - 检查文件是否存在
  - 读取文件内容
  - 根据文件扩展名确定 MIME 类型
  - 转换为 base64 编码
  - 返回 data URL 格式

## 2. 前端实现 (TypeScript/Vue)

### 2.1 使用 GreetService

前端可以通过导入 `GreetService` 来调用后端方法：

```typescript
import { GreetService } from "/root/bindings/dzhgo/index";

// 读取本地图片
async function loadLocalImage(filePath: string) {
	try {
		const base64Data = await GreetService.GetLocalImage(filePath);
		// base64Data 格式: "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ..."
		return base64Data;
	} catch (error) {
		console.error("读取本地图片失败:", error);
		throw error;
	}
}
```

### 2.2 在 Vue 组件中使用

```vue
<template>
	<div>
		<img :src="imageSrc" alt="本地图片" />
		<button @click="loadImage">加载图片</button>
	</div>
</template>

<script lang="ts" setup>
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { GreetService } from "/root/bindings/dzhgo/index";

const imageSrc = ref("");

async function loadImage() {
	const filePath = "/Users/username/Pictures/example.jpg";
	
	try {
		const base64Data = await GreetService.GetLocalImage(filePath);
		imageSrc.value = base64Data;
		ElMessage.success("图片加载成功");
	} catch (error) {
		ElMessage.error("图片加载失败: " + error);
	}
}
</script>
```

### 2.3 工具函数

在 `frontend/src/dzh/utils/imageConverter.ts` 中提供了工具函数：

```typescript
/**
 * 使用 Wails3 读取本地绝对路径图片并转换为 base64
 * @param filePath 本地图片的绝对路径
 * @returns Promise<string> base64 编码的图片数据
 */
export async function loadLocalImageAsBase64(filePath: string): Promise<string> {
	try {
		const { GreetService } = await import("/root/bindings/dzhgo/index");
		const base64Data = await GreetService.GetLocalImage(filePath);
		return base64Data;
	} catch (error) {
		console.error("读取本地图片失败:", error);
		throw new Error(`读取本地图片失败: ${error}`);
	}
}
```

## 3. 使用示例

### 3.1 基本用法

```typescript
// 直接使用 GreetService
const imagePath = "/Users/lizheng/Library/Application Support/dzhgo/public/uploads/20250705/c.jpg";
const base64Image = await GreetService.GetLocalImage(imagePath);
```

### 3.2 在表单中使用

```vue
<template>
	<el-form-item label="头像">
		<el-input v-model="imagePath" placeholder="输入图片路径" />
		<img v-if="imageSrc" :src="imageSrc" style="width: 100px; height: 100px;" />
		<el-button @click="loadLocalImage">加载图片</el-button>
	</el-form-item>
</template>

<script setup>
const imagePath = ref("");
const imageSrc = ref("");

async function loadLocalImage() {
	if (!imagePath.value) {
		ElMessage.warning("请输入图片路径");
		return;
	}
	
	try {
		imageSrc.value = await GreetService.GetLocalImage(imagePath.value);
		ElMessage.success("图片加载成功");
	} catch (error) {
		ElMessage.error("图片加载失败");
	}
}
</script>
```

## 4. 支持的图片格式

后端支持以下图片格式：
- `.jpg`, `.jpeg` - JPEG 格式
- `.png` - PNG 格式  
- `.gif` - GIF 格式
- `.webp` - WebP 格式
- 其他格式默认按 JPEG 处理

## 5. 错误处理

### 5.1 常见错误

1. **文件不存在**: 路径错误或文件不存在
2. **权限不足**: 没有读取文件的权限
3. **文件损坏**: 图片文件损坏或格式不支持

### 5.2 错误处理示例

```typescript
async function safeLoadImage(filePath: string) {
	try {
		const base64Data = await GreetService.GetLocalImage(filePath);
		return { success: true, data: base64Data };
	} catch (error) {
		return { 
			success: false, 
			error: error.message || "未知错误" 
		};
	}
}

// 使用
const result = await safeLoadImage("/path/to/image.jpg");
if (result.success) {
	imageSrc.value = result.data;
} else {
	ElMessage.error(`加载失败: ${result.error}`);
}
```

## 6. 性能考虑

1. **大文件处理**: 对于大图片文件，建议添加文件大小检查
2. **缓存机制**: 可以添加缓存来避免重复读取相同文件
3. **异步加载**: 使用异步方式避免阻塞 UI

## 7. 安全注意事项

1. **路径验证**: 验证文件路径，防止访问系统敏感文件
2. **文件类型检查**: 确保只读取图片文件
3. **大小限制**: 限制文件大小，防止内存溢出

## 8. 完整示例

参考 `frontend/src/modules/base/views/info.vue` 文件中的实现，展示了如何在用户信息页面中加载本地头像图片。 