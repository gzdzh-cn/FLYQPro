<template>
    <div class="clue-detail">
        <!-- 右上角操作 -->
        <div class="detail-actions">
            <el-tooltip content="刷新全部数据" placement="bottom">
                <button type="button" class="detail-action-btn" :class="{ 'is-refreshing': refreshingAll }"
                    :disabled="refreshingAll" aria-label="刷新全部数据" @click="refreshAllData">
                    <el-icon :size="18">
                        <RefreshRight />
                    </el-icon>
                </button>
            </el-tooltip>
            <el-tooltip v-if="!isFullscreen" content="关闭" placement="bottom">
                <button type="button" class="detail-action-btn" aria-label="关闭" @click="closeDrawer">
                    <el-icon :size="18">
                        <Close />
                    </el-icon>
                </button>
            </el-tooltip>
        </div>

        <!-- 顶部卡片 -->
        <div v-show="!isFullscreen" class="clue-info-card">
            <!-- 头像 -->
            <div class="card-left">
                <el-avatar :size="64" :src="ownerAvatar || '/customerPro/usreicon_80.png'" shape="square"
                    class="clickable-avatar" @click="avatarPreviewVisible = true" />
            </div>

            <!-- 内容区 -->
            <div class="card-right">
                <!-- 第一行：名称 + 操作按钮 -->
                <div class="row-top">
                    <span class="name">{{ cluesInfo.keywords || cluesInfo.projectName || '' }}</span>

                    <el-button type="success" size="small" round class="detail-call-anchor"
                        @mousedown.prevent @click="handleCall($event)">
                        <el-icon>
                            <Phone />
                        </el-icon> 打电话
                    </el-button>

                    <el-button v-if="!isReadonly" size="small" round text @click="editInfoRef?.open()">
                        <el-icon>
                            <EditPen />
                        </el-icon> 编辑
                    </el-button>

                    <el-button v-if="!isReadonly" size="small" round text @click="tagManagerRef?.open()">
                        <el-icon>
                            <PriceTag />
                        </el-icon> 标签
                    </el-button>

                    <el-popconfirm v-if="isOcean" title="确定认领该线索吗?" @confirm="handleClaim">
                        <template #reference>
                            <el-button type="warning" size="small" round text>
                                <el-icon>
                                    <Select />
                                </el-icon> 认领
                            </el-button>
                        </template>
                    </el-popconfirm>

                    <el-popconfirm v-if="!isOcean && cluesInfo.status == 0 && hasPushCommonClausePermission" title="确定放入公海吗?"
                        @confirm="pushCommonClause">
                        <template #reference>
                            <el-button type="warning" size="small" round text>
                                <el-icon>
                                    <Promotion />
                                </el-icon> 放入公海
                            </el-button>
                        </template>
                    </el-popconfirm>
                </div>

                <!-- 第二行：负责人 + 转交 + 参与人 -->
                <div class="row-mid">
                    <span class="label">负责人：</span>
                    <el-avatar :size="22" :src="''"><el-icon>
                            <UserFilled />
                        </el-icon></el-avatar>
                    <span class="val primary">{{ ownerDisplayName }}</span>

                    <el-button v-if="showTransferBtn" text size="small" @click="handleTransfer">
                        <el-icon>
                            <Promotion />
                        </el-icon> 转交
                    </el-button>

                    <span class="label">参与人：</span>
                    <template v-for="(p, idx) in participantList" :key="idx">
                        <el-avatar :size="22" :src="''"><el-icon>
                                <UserFilled />
                            </el-icon></el-avatar>
                        <span class="val primary">{{ p.name }}</span>
                    </template>
                    <el-icon class="action-icon" style="margin-left: 2px;" @click="participantRef?.open()">
                        <EditPen />
                    </el-icon>
                </div>
            </div>

            <!-- 第三行：标签 -->
            <div class="row-tags" v-if="tagList.length">
                <el-tag v-for="(tag, idx) in tagList" :key="idx" :effect="tag.effect" size="large"
                    :style="tag.style" :title="tag.typeName">{{ tag.text }}</el-tag>
            </div>
        </div>

        <!-- 下半部分：左右两栏 -->
        <div class="detail-body"
            :class="{ 'fullscreen-left': isFullscreen && fullscreenSide === 'left', 'fullscreen-right': isFullscreen && fullscreenSide === 'right' }">
            <!-- 左侧：客户资料 / 订单 -->
            <div class="panel-left">
                <div class="panel-header">
                    <el-radio-group v-model="leftTab" size="small" class="tab-group">
                        <el-radio-button label="profile">客户资料</el-radio-button>
                        <el-radio-button v-if="!isOcean" label="order">订单</el-radio-button>
                    </el-radio-group>
                    <div class="header-right">
                        <el-icon :size="16" @click="toggleFullscreen" v-if="!isFullscreen">
                            <FullScreen />
                        </el-icon>
                        <el-icon :size="16" @click="emitFullscreen" v-else>
                            <ScaleToOriginal />
                        </el-icon>
                    </div>
                </div>

                <!-- 客户资料内容 -->
                <div v-show="leftTab === 'profile'" class="panel-content">
                    <!-- 上部分：基本信息 / 更多信息 -->
                    <div class="sub-tab-bar">
                        <span :class="{ active: subTab === 'basic' }" @click="subTab = 'basic'">
                            <el-icon class="collapse-icon" @click.stop="toggleCollapse">
                                <ArrowDown v-if="!collapsed" />
                                <ArrowRight v-else />
                            </el-icon> 基本信息
                        </span>
                        <!-- <span :class="{ active: subTab === 'more' }" @click="subTab = 'more'">更多信息<el-icon
                                class="setting-icon" @click.stop>
                                <Setting />
                            </el-icon></span> -->
                    </div>

                    <!-- 基本信息 -->
                    <div v-show="subTab === 'basic' && !collapsed" class="info-grid">
                        <div v-if="isAdmin" class="info-row">
                            <div class="info-left">
                                <label>ID：</label>
                                <span>{{ cluesInfo.id }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>53标识：</label>
                                <span>{{ profileData.guestId }}</span>
                            </div>
                            <div class="info-right">
                                <label>跟进记录：</label><span class="val-red">{{ profileData.followCount }}</span>
                                <el-icon v-if="!isOcean" class="action-icon" @click="openFollowDialog">
                                    <Plus />
                                </el-icon>

                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>跟进任务：</label>
                                <span class="val-red">{{ profileData.taskCount }}</span>
                                <el-icon v-if="!isOcean" class="action-icon" @click="ElMessage.info('功能正在开发')">
                                    <Plus />
                                </el-icon>
                            </div>
                            <div class="info-right">
                                <label>成交次数：</label><span class="val-red">{{ profileData.dealCount }}</span>
                                <el-icon v-if="!isOcean" class="action-icon" @click="openOrderAdd">
                                    <Plus />
                                </el-icon>
                            </div>
                        </div>
                        <!-- <div class="info-row">
                            <div class="info-left">
                                <label>成交总额：</label>
                                <span>{{ profileData.dealAmount }}</span>
                                <el-icon class="icon-gray">
                                    <View />
                                </el-icon>
                                <el-icon color="var(--color-primary)">
                                    <Plus />
                                </el-icon>
                            </div>
                            <div class="info-right">
                                <label>总利润：</label><span>{{ profileData.profit }}</span>
                            </div>
                        </div> -->
                        <div class="info-row">
                            <div class="info-left">
                                <label>录入时间：</label>
                                <span class="link-text">{{ profileData.createTime }}</span>
                            </div>
                            <!-- <div class="info-right">
                                <label>相关文档：</label><a href="#">{{ profileData.docCount }}</a>
                                <el-icon>
                                    <EditPen />
                                </el-icon>
                            </div> -->
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>最后跟进时间：</label>
                                <span class="link-text">{{ profileData.lastFollowTime }}
                                    <!-- <el-icon class="icon-warn">
                                        <QuestionFilled />
                                    </el-icon> -->
                                </span>
                            </div>
                            <div class="info-right">
                                <label>最后跟进人：</label>
                                <el-avatar :size="18" :src="''"><el-icon>
                                        <UserFilled />
                                    </el-icon></el-avatar>
                                {{ profileData.lastFollower }}
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>下次跟进时间：</label>
                                <span>{{ profileData.nextFollowTime }}</span>
                                <!-- <el-icon class="action-icon">
                                    <Plus />
                                </el-icon> -->
                                <!-- <el-icon class="icon-warn">
                                    <QuestionFilled />
                                </el-icon> -->
                            </div>
                            <div class="info-right" v-if="cluesInfo.createdId">
                                <label>录入人：</label>
                                <el-avatar :size="18" :src="''"><el-icon>
                                        <UserFilled />
                                    </el-icon></el-avatar>
                                {{ createdDisplayName }}
                            </div>
                        </div>
                        <div v-if="String(cluesInfo.filterRemark || '').trim()" class="info-row">
                            <div class="info-left">
                                <label>过滤原因：</label>
                                <span>{{ cluesInfo.filterRemark }}</span>
                            </div>
                        </div>
                    </div>

                    <!-- 折叠占位 -->
                    <div v-show="collapsed" class="collapse-placeholder" @click="toggleCollapse">
                        <el-icon :size="24" class="icon-double-down">
                            <DArrowRight />
                        </el-icon>
                    </div>

                    <!-- 更多信息 -->
                    <div v-show="subTab === 'more' && !collapsed" class="info-grid">
                        <div class="info-row">
                            <div class="info-left">
                                <label>数据来源：</label>
                                <span>{{ moreInfo.dataSource }}</span>
                            </div>
                            <div class="info-right">
                                <label>成交产品总数：</label>
                                <span>{{ moreInfo.productCount }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>收款比例：</label>
                                <span>{{ moreInfo.receiptRatio }}</span>
                            </div>
                            <div class="info-right">
                                <label>开票比例：</label>
                                <span>{{ moreInfo.invoiceRatio }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>未开票金额：</label>
                                <span>{{ moreInfo.uninvoiceAmount }}</span>
                            </div>
                            <div class="info-right">
                                <label>合同总数：</label>
                                <span>{{ moreInfo.contractCount }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>合同总金额：</label>
                                <span>{{ moreInfo.contractAmount }}</span>
                            </div>
                            <div class="info-right">
                                <label>合同最后有效结束时间：</label>
                                <span>{{ moreInfo.contractEndTime }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>合同最后状态：</label>
                                <span>{{ moreInfo.contractStatus }}</span>
                            </div>
                            <div class="info-right">
                                <label>最后修改时间：</label>
                                <span>{{ moreInfo.lastModifyTime }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>修改次数：</label>
                                <span>{{ moreInfo.modifyCount }}</span>
                            </div>
                            <div class="info-right">
                                <label>最后成交时间：</label>
                                <span>{{ moreInfo.lastDealTime }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>订单到期时间：</label>
                                <span>{{ moreInfo.orderExpireTime }}</span>
                            </div>
                            <div class="info-right">
                                <label>最后跟进阶段：</label>
                                <span>{{ moreInfo.lastFollowStage }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>成交周期：</label>
                                <span>{{ moreInfo.dealCycle }}</span>
                            </div>
                            <div class="info-right">
                                <label>首次成交时间：</label>
                                <span>{{ moreInfo.firstDealTime }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>首次跟进时间：</label>
                                <span>{{ moreInfo.firstFollowTime }}</span>
                            </div>
                            <div class="info-right">
                                <label>首次跟进时长：</label>
                                <span>{{ moreInfo.firstFollowDuration }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>首次跟进人：</label>
                                <span>{{ moreInfo.firstFollower }}</span>
                            </div>
                            <div class="info-right">
                                <label>联系人总数：</label>
                                <span>{{ moreInfo.contactCount }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>客户模板：</label>
                                <span>{{ moreInfo.customerTemplate }}</span>
                            </div>
                            <div class="info-right">
                                <label>评论数：</label>
                                <span>{{ moreInfo.commentCount }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left">
                                <label>首次负责人：</label>
                                <span>{{ moreInfo.firstOwner }}</span>
                            </div>
                        </div>
                    </div>

                    <!-- 下部分：个人信息 -->
                    <div class="section-divider"></div>
                    <div class="sub-tab-bar">
                        <span :class="{ active: true }">
                            <el-icon class="collapse-icon" @click.stop="togglePersonalCollapse">
                                <ArrowDown v-if="!personalCollapsed" />
                                <ArrowRight v-else />
                            </el-icon> 个人信息
                        </span>
                        <div class="hide-empty-switch">
                            <el-switch v-model="hideEmpty" size="small" />
                            <el-tooltip content="隐藏没有内容的信息" placement="top">
                                <el-icon :size="14" class="question-icon">
                                    <QuestionFilled />
                                </el-icon>
                            </el-tooltip>
                        </div>
                    </div>
                    <div v-show="!personalCollapsed" class="info-grid">
                        <!-- <div class="info-row"> -->
                            <!-- <div class="info-left" v-show="!hideEmpty || ownerDisplayName">
                                <label>负责人：</label>
                                <span>{{ ownerDisplayName }}</span>
                            </div> -->

                        <!-- </div> -->
                        <div class="info-row">
                            <!-- <div class="info-left" v-show="!hideEmpty || personalInfo.guestId">
                                <label>53标识：</label>
                                <span>{{ personalInfo.guestId }}</span>
                            </div> -->
                            <div class="info-right" v-show="!hideEmpty || personalInfo.getTime">
                                <label>获得客户时间：</label>
                                <span class="link-text">{{ personalInfo.getTime }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.projectName">
                                <label>项目：</label>
                                <span>{{ personalInfo.projectName }}</span>
                            </div>
                        </div>
                        <!-- <div class="info-row"> -->
                            <!-- <div class="info-left" v-show="!hideEmpty || personalInfo.name">
                                <label>姓名：</label>
                                <span>{{ personalInfo.name }}</span>
                            </div> -->
                            
                        <!-- </div> -->
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.mobile">
                                <label>手机号：</label>
                                <span>{{ personalInfo.mobile }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.wechat">
                                <label>微信号：</label>
                                <span>{{ personalInfo.wechat }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.emergencyMobile">
                                <label>紧急联系人电话：</label>
                                <span>{{ personalInfo.emergencyMobile }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.gender">
                                <label>性别：</label>
                                <span>{{ personalInfo.gender }}</span>
                            </div>
                            <!-- <div class="info-right" v-show="!hideEmpty || personalInfo.status">
                                <label>客户状态：</label>
                                <span>{{ personalInfo.status }}</span>
                            </div> -->
                        </div>

                        
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.householdAddress">
                                <label>户籍地址：</label>
                                <span>{{ personalInfo.householdAddress }}</span>
                            </div>
                            <div class="info-left" v-show="!hideEmpty || personalInfo.guestIpInfo">
                                <label>IP归属地：</label>
                                <span>{{ personalInfo.guestIpInfo }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.keywords">
                                <label>关键词：</label>
                                <span>{{ personalInfo.keywords }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.pushSchool">
                                <label>已推学校：</label>
                                <span>{{ personalInfo.pushSchool }}</span>
                            </div>
                        </div>

                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.graduatedSchool">
                                <label>毕业院校：</label>
                                <span>{{ personalInfo.graduatedSchool }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.schoolName">
                                <label>意向院校：</label>
                                <span>{{ personalInfo.schoolName }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.majorsName">
                                <label>意向专业：</label>
                                <span>{{ personalInfo.majorsName }}</span>
                            </div>
                            <div class="info-right" v-show="!hideEmpty || personalInfo.majorsType">
                                <label>报读类型：</label>
                                <span>{{ personalInfo.majorsType }}</span>
                            </div>
                        </div>
                        <div class="info-row">
                            <div class="info-left" v-show="!hideEmpty || personalInfo.degreeName">
                                <label>报读层次：</label>
                                <span>{{ personalInfo.degreeName }}</span>
                            </div>
                           
                        </div>
                        <!-- 动态标签类型字段：根据字典类型自动渲染 -->
                        <template v-for="dt in dictTagFields" :key="dt.key">
                            <div class="info-row" v-show="!hideEmpty || dt.value">
                                <div class="info-left">
                                    <label>{{ dt.label }}：</label>
                                    <span>{{ dt.value }}</span>
                                    <el-icon class="tag-edit-icon" @click="openTagEdit(dt)"><EditPen /></el-icon>
                                </div>
                            </div>
                        </template>
                        <div class="info-row">
                            <div class="info-right" v-show="!hideEmpty || personalInfo.remark">
                                <label>备注：</label>
                                <span>{{ personalInfo.remark }}</span>
                            </div>
                        </div>
                    </div>
                    <!-- 个人信息折叠占位 -->
                    <div v-show="personalCollapsed" class="collapse-placeholder" @click="togglePersonalCollapse">
                        <el-icon :size="24" class="icon-double-down">
                            <DArrowRight />
                        </el-icon>
                    </div>
                </div>

                <!-- 订单内容 -->
                <div v-if="leftTab === 'order'" class="panel-content">
                    <OrderList ref="orderListRef" :cluesId="cluesId" @success="onOrderSuccess" />
                </div>
            </div>


            <div class="panel-right">
                <div class="panel-header">
                    <el-radio-group v-model="rightTab" size="small" class="tab-group">
                        <el-radio-button label="followup">联系跟进</el-radio-button>
                        <!-- <el-radio-button label="assistant">助手</el-radio-button> -->
                        <el-radio-button label="track" v-if="service.customer_pro.clues._permission.getTrackList">日志轨迹</el-radio-button>
                        <el-radio-button label="chat" v-if="hasChatContentListPermission">聊天记录</el-radio-button>
                    </el-radio-group>
                    <div class="header-right">
                        <el-icon :size="16" @click="toggleRightFullscreen" v-if="!isFullscreen">
                            <FullScreen />
                        </el-icon>
                        <el-icon :size="16" @click="emitFullscreen" v-else>
                            <ScaleToOriginal />
                        </el-icon>
                        <el-icon :size="16">
                            <Search />
                        </el-icon>
                        <el-icon :size="16">
                            <Setting />
                        </el-icon>
                    </div>
                </div>

                <!-- 联系跟进内容 -->
                <div v-show="rightTab === 'followup'" class="panel-content followup-panel">
                    <!-- 快速跟进输入区（公海不显示） -->
                    <div v-if="!isOcean" class="quick-follow" :class="{ expanded: followExpanded }">
                        <el-input
                            v-model="followForm.remark"
                            type="textarea"
                            :autosize="{ minRows: 1, maxRows: 4 }"
                            placeholder="快速写跟进..."
                            @focus="followInputFocused = true"
                            @blur="onFollowInputBlur"
                        />
                        <div class="quick-follow-footer" :class="{ visible: followExpanded }">
                            <div class="footer-left">
                                <el-date-picker
                                    v-model="followForm.nextFollowupTime"
                                    type="datetime"
                                    :default-time="defaultTime"
                                    value-format="YYYY-MM-DD HH:mm"
                                    time-format="HH:mm"
                                    :disabled-date="disabledDate"
                                    placeholder="下次跟进日期时间"
                                    class="next-follow-picker"
                                    @visible-change="onDatePickerVisibleChange"
                                />
                            </div>
                            <div class="footer-right">
                                <el-button size="small" @click="cancelFollowInput">取消</el-button>
                                <el-button type="primary" size="small" :loading="followSaving" @click="submitFollow">保存</el-button>
                            </div>
                        </div>
                    </div>

                    <!-- 跟进记录列表 -->
                    <sub-follow-list ref="followListRef" :cluesId="cluesId" :readonly="isOcean" />
                </div>

                <!-- 助手内容 -->
                <div v-show="rightTab === 'assistant'" class="panel-content">
                    <el-empty description="暂无助手数据" />
                </div>

                <!-- 轨迹内容 -->
                <div v-show="rightTab === 'track'" class="panel-content">
                    <sub-track ref="trackRef" :id="cluesId ? String(cluesId) : undefined" />
                </div>

                <!-- 聊天记录内容 -->
                <div v-show="rightTab === 'chat'" class="panel-content">
                    <sub-chat ref="chatRef" :cluesId="cluesId" />
                </div>
            </div>
        </div>

        <!-- 编辑个人信息弹窗 -->
        <editInfo ref="editInfoRef" :cluesId="cluesId" @saved="onEditSaved" />

        <!-- 标签管理弹窗 -->
        <tagManager ref="tagManagerRef" :cluesInfo="cluesInfo" :cluesId="cluesId" @saved="onTagManagerSaved" />

        <!-- 参与人管理弹窗 -->
        <participantDialog ref="participantRef" :cluesInfo="cluesInfo" :cluesId="cluesId" @saved="loadData" />

        <!-- 转交弹窗 -->
        <el-dialog v-model="transferVisible" title="转交线索" width="480px" append-to-body :close-on-click-modal="false">
            <el-form label-width="80px">
                <el-form-item label="项目">
                    <el-select v-model="transferForm.projectId" @change="onTransferProjectChange" placeholder="请选择项目" style="width: 100%">
                        <el-option v-for="item in transferProjectList" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                </el-form-item>
                <el-form-item label="客服组">
                    <el-select v-model="transferForm.groupId" @change="onTransferGroupChange" placeholder="请选择客服组" style="width: 100%">
                        <el-option v-for="item in transferGroupList" :key="item.id" :label="item.name" :value="item.id" />
                    </el-select>
                </el-form-item>
                <el-form-item label="接收人">
                    <el-select v-model="transferForm.servicesId" placeholder="请选择接收人" style="width: 100%">
                        <el-option v-for="item in transferKfList" :key="item.userId" :label="item.name" :value="item.userId" />
                    </el-select>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="transferVisible = false">取消</el-button>
                <el-button type="primary" :loading="transferSaving" @click="onTransferSubmit">确认转交</el-button>
            </template>
        </el-dialog>

        <!-- 单个标签类型编辑弹窗 -->
        <el-dialog v-model="tagEditVisible" :title="'编辑' + tagEditState.label" width="400px" append-to-body :close-on-click-modal="false">
            <el-select v-if="tagEditState.isMulti" v-model="tagEditState.selectedMulti" multiple filterable placeholder="输入关键字搜索" style="width: 100%">
                <el-option v-for="opt in tagEditState.options" :key="opt.value" :label="opt.name" :value="opt.value" />
            </el-select>
            <el-select v-else v-model="tagEditState.selectedSingle" filterable placeholder="输入关键字搜索" style="width: 100%">
                <el-option v-for="opt in tagEditState.options" :key="opt.value" :label="opt.name" :value="opt.value" />
            </el-select>
            <template #footer>
                <el-button @click="tagEditVisible = false">取消</el-button>
                <el-button type="primary" :loading="tagEditSaving" @click="saveTagEdit">保存</el-button>
            </template>
        </el-dialog>

        <!-- 头像预览 -->
        <el-image-viewer v-if="avatarPreviewVisible" :url-list="[ownerAvatar || '/customerPro/usreicon_80.png']"
            @close="avatarPreviewVisible = false" />
    </div>
</template>

<script lang="ts" name="clues-info" setup>
import { ref, onMounted, computed, reactive, nextTick, watch } from "vue";
import { useCool } from "/@/cool";
import { useBase } from "/$/base";
import { ElMessage } from "element-plus";
import dayjs from "dayjs";
import editInfo from "./editInfo.vue";
import tagManager from "./tagManager.vue";
import { deviceState } from "/@/utils/deviceWs";
import SubTrack from "./subTrack.vue";
import SubFollowList from "./subFollowList.vue";
import SubChat from "./subChat.vue";
import ParticipantDialog from "./participantDialog.vue";
import OrderList from "./orderList.vue";
import {
    Phone,
    UserFilled,
    Promotion,
    FullScreen,
    ScaleToOriginal,
    Setting,
    Plus,
    EditPen,
    QuestionFilled,
    Search,
    ArrowDown,
    ArrowRight,
    DArrowRight,
    Close,
    RefreshRight,
    PriceTag,
    Select
} from "@element-plus/icons-vue";
import { useDictMeta, resolveTags, dictLabel as _dictLabel, toValueArray, parseLabelJson, KNOWN_DB_FIELDS, FIELD_TO_TYPE_KEY } from "../../utils/tagDict";

const props = defineProps<{
    cluesId?: string | number;
    /** 模式：默认私海(可编辑)，ocean=公海(只读+可认领)，filter=筛选(只读) */
    mode?: "ocean" | "filter";
}>();

const isOcean = computed(() => props.mode === "ocean");
const isReadonly = computed(() => !!props.mode);

const { service } = useCool();
const { user } = useBase();

// 转交按钮权限控制：超管始终可见；负责人是自己可见；自己是负责人的组长(role=2同组)或部门主管(role=3同项目)也可见
const userInfo = ref();
const isAdmin = ref(false);
const isOwner = computed(() => {
    const currentUserId = userInfo.value?.id;
    const servicesId = cluesInfo.value?.servicesId;
    return currentUserId && servicesId && String(currentUserId) === String(servicesId);
});
// 当前用户在 kfList 中的信息
const currentKf = computed(() => {
    const currentUserId = userInfo.value?.id;
    if (!currentUserId || kfList.value.length === 0) return null;
    return kfList.value.find((e: any) => String(e.userId) === String(currentUserId)) || null;
});
// 线索负责人在 kfList 中的信息
const ownerKf = computed(() => {
    const servicesId = cluesInfo.value?.servicesId;
    if (!servicesId || kfList.value.length === 0) return null;
    return kfList.value.find((e: any) => String(e.userId) === String(servicesId)) || null;
});
// 是否是负责人的组长或部门主管
const isOwnerSupervisor = computed(() => {
    if (!currentKf.value || !ownerKf.value) return false;
    // 组长：role=2 且与负责人同组
    if (currentKf.value.role === 2 && currentKf.value.groupId === ownerKf.value.groupId) return true;
    // 部门主管：role=3 且与负责人同项目
    if (currentKf.value.role === 3 && currentKf.value.projectId === ownerKf.value.projectId) return true;
    return false;
});
const showTransferBtn = computed(() => isAdmin.value || isOwner.value || isOwnerSupervisor.value);
const getUserInfo = async () => {
    userInfo.value = await service.customer_pro.comm.userInfo();
    isAdmin.value = user.info.roleIds?.split(",").includes("1") || false;
};

// 编辑弹窗引用
const editInfoRef = ref<InstanceType<typeof editInfo>>();
// 标签管理弹窗引用
const tagManagerRef = ref<InstanceType<typeof tagManager>>();
// 参与人管理弹窗引用
const participantRef = ref<InstanceType<typeof ParticipantDialog>>();
// 订单列表引用
const orderListRef = ref<InstanceType<typeof OrderList>>();
// 轨迹列表引用
const trackRef = ref<InstanceType<typeof SubTrack>>();

// ===== 单个标签类型编辑 =====
const tagEditVisible = ref(false);
const tagEditSaving = ref(false);
const tagEditState = reactive({
    label: "",
    fieldName: "",
    typeId: "",
    isMulti: false,
    selectedMulti: [] as string[],
    selectedSingle: "" as string,
    options: [] as { value: string; name: string }[]
});

function openTagEdit(dt: typeof dictTagFields.value[number]) {
    tagEditState.label = dt.label;
    tagEditState.fieldName = dt.fieldName;
    tagEditState.typeId = dt.typeId;
    tagEditState.isMulti = dt.isMulti;
    tagEditState.selectedMulti = [...dt.rawValues];
    tagEditState.selectedSingle = dt.rawValues[0] || "";
    tagEditState.options = dictItemsByType.value[dt.typeId] || [];
    tagEditVisible.value = true;
}

async function saveTagEdit() {
    tagEditSaving.value = true;
    try {
        const val = tagEditState.isMulti
            ? (tagEditState.selectedMulti.length ? tagEditState.selectedMulti : null)
            : (tagEditState.selectedSingle || null);

        // 构建更新数据：带上所有标签类型字段的当前值，避免后端 ModifyAfter 清空未传的字段
        const updateData: Record<string, any> = { id: props.cluesId };
        const customLabels: Record<string, any> = {};
        const existingLabelJson = parseLabelJson(cluesInfo.value?.labelJson);

        dictTagFields.value.forEach((dt) => {
            if (dt.fieldName === tagEditState.fieldName) {
                // 当前编辑的字段
                if (KNOWN_DB_FIELDS.has(dt.fieldName)) {
                    // 已知字段：多选数组转 JSON 字符串，单选直接传值
                    if (dt.isMulti && Array.isArray(val) && val.length) {
                        updateData[dt.fieldName] = JSON.stringify(val);
                    } else {
                        updateData[dt.fieldName] = val;
                    }
                } else {
                    // 自定义字段：存入 labelJson（单选传字符串，多选传数组）
                    if (val !== null && val !== undefined && val !== "") {
                        if (dt.isMulti && Array.isArray(val) && val.length) {
                            customLabels[dt.key] = val;
                        } else {
                            customLabels[dt.key] = val;
                        }
                    }
                }
            } else {
                // 保留其他标签类型字段的当前值
                if (KNOWN_DB_FIELDS.has(dt.fieldName)) {
                    if (dt.isMulti) {
                        // 多选已知字段：数组转 JSON 字符串
                        updateData[dt.fieldName] = dt.rawValues.length ? JSON.stringify(dt.rawValues) : "";
                    } else {
                        // 单选已知字段：取第一个值
                        updateData[dt.fieldName] = dt.rawValues.length ? dt.rawValues[0] : "";
                    }
                } else {
                    // 自定义字段：保留到 labelJson（单选存字符串，多选存数组）
                    if (dt.rawValues.length) {
                        if (dt.isMulti) {
                            customLabels[dt.key] = dt.rawValues;
                        } else {
                            customLabels[dt.key] = dt.rawValues[0];
                        }
                    }
                }
            }
        });

        // 合并 labelJson
        const merged = { ...existingLabelJson, ...customLabels };
        // 清除被清空的自定义字段
        dictTagFields.value.forEach((dt) => {
            if (!KNOWN_DB_FIELDS.has(dt.fieldName) && !customLabels[dt.key]) {
                delete merged[dt.key];
            }
        });
        if (Object.keys(merged).length > 0) {
            updateData.labelJson = JSON.stringify(merged);
        } else if (Object.keys(existingLabelJson).length > 0) {
            // 原来有 labelJson 但现在清空了
            updateData.labelJson = "";
        }

        await (service.customer_pro.clues as any).update(updateData);
        ElMessage.success("保存成功");
        tagEditVisible.value = false;
        loadData();
    } catch (e: any) {
        ElMessage.error("保存失败：" + (e.message || ""));
    } finally {
        tagEditSaving.value = false;
    }
}

// ===== 来源/学历映射已迁移至字典 =====

// ===== 线索数据 =====
const cluesInfo = ref<any>({});

// ===== 负责人显示名称 =====
const kfList = ref<any[]>([]);
const ownerDisplayName = computed(() => {
    // 通过 servicesId 从 kfList 中查找会员名称
    if (cluesInfo.value.servicesId && kfList.value.length > 0) {
        const kf = kfList.value.find((e: any) => e.userId === cluesInfo.value.servicesId);
        if (kf) return kf.name;
    }
    // 兜底：使用 servicesNames（API 可能直接返回）
    return cluesInfo.value.servicesNames || '';
});

// ===== 录入人显示名称 =====
const createdDisplayName = computed(() => {
    if (cluesInfo.value.createdId && kfList.value.length > 0) {
        const kf = kfList.value.find((e: any) => e.userId === cluesInfo.value.createdId);
        if (kf) return kf.name;
    }
    return '';
});

// ===== 负责人头像 =====
const ownerAvatar = computed(() => {
    // 优先使用后端返回的 ownerHeadImg
    if (cluesInfo.value.ownerHeadImg) return cluesInfo.value.ownerHeadImg;
    // 兜底：通过 servicesId 从 kfList 中查找会员头像
    const userId = cluesInfo.value.servicesId;
    if (userId && kfList.value.length > 0) {
        const kf = kfList.value.find((e: any) => e.id === userId || e.userId === userId);
        if (kf?.headImg) return kf.headImg;
    }
    return '';
});

// ===== 参与人列表（通过 servicesIds 解析会员名称） =====
const participantList = computed(() => {
    const ids = cluesInfo.value.servicesIds;
    const ownerId = cluesInfo.value.servicesId;
    if (!ids) return [];
    const idArr: string[] = Array.isArray(ids) ? ids : String(ids).split(",").filter(Boolean);
    if (idArr.length === 0) return [];
    // 过滤掉负责人
    const filtered = idArr.filter((id: string) => id !== ownerId);
    return filtered.map((id: string) => {
        const kf = kfList.value.find((e: any) => e.userId === id);
        return { name: kf ? kf.name : id };
    });
});

// ===== 标签数据（使用公共模块） =====
const { dictTypes, dictItemsByType, loadDictMeta, resetDictMeta: resetLocalDictMeta } = useDictMeta();

// 通过字典类型 key 查找 value 对应的 name（代理到公共模块）
function dictLabel(typeKey: string, val: any): string {
    return _dictLabel(typeKey, val);
}

// 标签列表：根据线索数据中已填写、且能在字典中匹配到的字段值生成
const tagList = computed(() => resolveTags(cluesInfo.value || {}));

// ===== 左右面板 Tab 状态 =====
const leftTab = ref("profile");
const rightTab = ref("followup");
const subTab = ref("basic");
const collapsed = ref(false);
const personalCollapsed = ref(false);
const hideEmpty = ref(true);

// ===== 日期格式化：6月11日 (周四) 17点15分 =====
const weekdays = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
function fmtDate(v: string): string {
    if (!v) return "";
    const d = dayjs(v);
    if (!d.isValid()) return v;
    return `${d.month() + 1}月${d.date()}日 (${weekdays[d.day()]}) ${d.hour()}点${d.minute()}分`;
}

// ===== 客户资料数据 =====
const profileData = computed(() => ({
    code: cluesInfo.value.id || "",
    guestId: cluesInfo.value.guestId || "",
    followCount: cluesInfo.value.followCount || 0,
    taskCount: 0,
    dealCount: cluesInfo.value.dealCount || 0,
    dealAmount: 0,
    profit: 0,
    createTime: fmtDate(cluesInfo.value.createTime || ""),
    docCount: 0,
    lastFollowTime: cluesInfo.value.lastFollowupTime ? fmtDate(cluesInfo.value.lastFollowupTime) : "",
    lastFollower: cluesInfo.value.lastFollowupName || "",
    nextFollowTime: cluesInfo.value.nextFollowTime ? fmtDate(cluesInfo.value.nextFollowTime) : "未设置",
    creator: createdDisplayName.value
}));

// ===== 跟进记录列表引用 =====
const followListRef = ref<any>();

// ===== 快速跟进输入 =====
const followInputFocused = ref(false);
const followSaving = ref(false);
const followForm = ref({
    remark: "",
    nextFollowupTime: ""
});
const defaultTime = new Date();

function disabledDate(time: { getTime: () => number }) {
    return time.getTime() < Date.now() - 8.64e7;
}

const datePickerOpen = ref(false);

// 输入框是否保持展开：有内容/有日期/日期弹窗打开/正在聚焦
const followExpanded = computed(() => {
    return followInputFocused.value
        || datePickerOpen.value
        || !!followForm.value.remark?.trim()
        || !!followForm.value.nextFollowupTime;
});

function onDatePickerVisibleChange(visible: boolean) {
    datePickerOpen.value = visible;
}

function onFollowInputBlur() {
    setTimeout(() => {
        // 日期弹窗打开时保持展开
        if (datePickerOpen.value) return;
        // 检查焦点是否移到了 footer 内的元素（日期选择器、保存按钮）
        const activeEl = document.activeElement;
        const followBox = document.querySelector('.quick-follow');
        if (followBox && activeEl && followBox.contains(activeEl)) return;
        followInputFocused.value = false;
    }, 300);
}

function cancelFollowInput() {
    followForm.value = { remark: "", nextFollowupTime: "" };
    followInputFocused.value = false;
}

async function submitFollow() {
    if (!followForm.value.remark?.trim()) {
        ElMessage.error("请填写跟进内容");
        return;
    }
    if (!followForm.value.nextFollowupTime) {
        ElMessage.error("请选择下次跟进时间");
        return;
    }
    followSaving.value = true;
    try {
        const submitData: any = {
            cluesId: props.cluesId,
            remark: followForm.value.remark
        };
        if (followForm.value.nextFollowupTime) {
            submitData.nextFollowupTime = followForm.value.nextFollowupTime;
        }
        await (service.customer_pro.clues as any).followAdd(submitData);
        ElMessage.success("跟进保存成功");
        followForm.value = { remark: "", nextFollowupTime: "" };
        followInputFocused.value = false;
        // 刷新跟进记录列表
        followListRef.value?.refresh();
    } catch (e: any) {
        ElMessage.error("保存失败：" + (e.message || ""));
    } finally {
        followSaving.value = false;
    }
}

// ===== 跟进记录弹窗 =====
function openFollowDialog() {
    followListRef.value?.openFollowDialog();
}

// ===== 更多信息数据 =====
const moreInfo = computed(() => ({
    dataSource: dictLabel("sourceFrom", cluesInfo.value.sourceFrom),
    productCount: 0,
    receiptRatio: "0%",
    invoiceRatio: "0%",
    uninvoiceAmount: 0,
    contractCount: 0,
    contractAmount: 0,
    contractEndTime: "",
    contractStatus: "",
    lastModifyTime: fmtDate(cluesInfo.value.updateTime || ""),
    modifyCount: 0,
    lastDealTime: "",
    orderExpireTime: "",
    lastFollowStage: cluesInfo.value.followupType ? formatFollowupType(cluesInfo.value.followupType) : "",
    dealCycle: "",
    firstDealTime: "",
    firstFollowTime: cluesInfo.value.firstFollowTime ? fmtDate(cluesInfo.value.firstFollowTime) : "",
    firstFollowDuration: "",
    firstFollower: cluesInfo.value.firstFollower || "",
    contactCount: 0,
    customerTemplate: "客户",
    commentCount: 0,
    firstOwner: ownerDisplayName.value
}));

// ===== ID→名称翻译缓存 =====
const schoolMap = ref<Map<string, string>>(new Map());
const majorsMap = ref<Map<string, string>>(new Map());
const readtypesMap = ref<Map<string, string>>(new Map());
const readdegreeMap = ref<Map<string, string>>(new Map());

async function loadNameMaps() {
    try {
        const [schoolList, majorsList, readtypesList, readdegreeList] = await Promise.all([
            service.customer_pro.school.list().catch(() => []),
            service.customer_pro.majors.list({}).catch(() => []),
            service.customer_pro.readtypes.list().catch(() => []),
            service.customer_pro.readdegree.list().catch(() => [])
        ]);
        schoolMap.value = new Map((schoolList || []).map((s: any) => [String(s.id), s.name]));
        majorsMap.value = new Map((majorsList || []).map((m: any) => [String(m.id), m.name]));
        readtypesMap.value = new Map((readtypesList || []).map((r: any) => [String(r.id), r.name]));
        readdegreeMap.value = new Map((readdegreeList || []).map((r: any) => [String(r.id), r.name]));
    } catch (e) {
        console.error("加载名称映射失败:", e);
    }
}

// ===== 个人信息数据 =====
const personalInfo = computed(() => {
    // 性别（固定选项）
    const genderMap: Record<string, string> = { "0": "保密", "1": "男", "2": "女" };

    // ID→名称翻译
    const info = cluesInfo.value || {};
    const schoolNameById = schoolMap.value.get(String(info.schoolId)) || "";
    const majorsNameById = majorsMap.value.get(String(info.majorsId)) || "";
    const majorsTypeNameById = readtypesMap.value.get(String(info.majorsType)) || "";
    const degreeNameById = readdegreeMap.value.get(String(info.degreeId)) || "";

    return {
        getTime: fmtDate(info.allotTime || ""),
        guestId: info.guestId || "",
        projectName: info.projectName || "",
        name: info.name || "",
        gender: genderMap[info.gender] || "",
        mobile: info.mobile || "",
        wechat: info.wechat || "",
        emergencyMobile: info.emergencyMobile || "",
        status: info.status == 1 ? "已成交" : "未成交",
        source: dictLabel("sourceFrom", info.sourceFrom),
        eduForm: dictLabel("education", info.education),
        graduatedSchool: info.graduatedSchool || "",
        schoolName: schoolNameById,
        majorsName: majorsNameById,
        majorsType: majorsTypeNameById,
        degreeName: degreeNameById,
        householdType: dictLabel("householdType", info.householdType),
        householdAddress: info.householdAddress || "",
        ip: info.ip || "",
        guestIpInfo: info.guestIpInfo || "",
        pushSchool: info.schoolName || "",
        keywords: info.keywords || "",
        remark: info.remark || ""
    };
});

// ===== 动态标签类型字段：根据字典类型自动渲染个人信息中的标签行 =====
const MULTI_SELECT_KEYS = new Set(["cluesLevel"]);

const dictTagFields = computed(() => {
    const info = cluesInfo.value || {};
    const fields: { key: string; label: string; value: string; fieldName: string; typeId: string; rawValues: string[]; isMulti: boolean }[] = [];

    // 反向映射：字典类型 key → 线索字段名
    const typeKeyToField: Record<string, string> = {};
    Object.entries(FIELD_TO_TYPE_KEY).forEach(([field, typeKey]) => {
        typeKeyToField[typeKey] = field;
    });

    // 解析 labelJson（自定义标签数据）
    const labelJson = parseLabelJson(info.labelJson);

    dictTypes.value.forEach((t) => {
        if (!t.key) return;
        // 字段名：优先从映射表取，否则直接用 key
        const fieldName = typeKeyToField[t.key] || t.key;
        // 优先从已知字段取值，其次从 labelJson 取值
        let raw = info[fieldName];
        if ((raw === undefined || raw === null || raw === "") && labelJson[t.key] !== undefined) {
            raw = labelJson[t.key];
        }
        if (raw === undefined || raw === null || raw === "") return;

        const values = toValueArray(raw);
        if (!values.length) return;

        // 将 value 翻译为 name
        const items = dictItemsByType.value[String(t.id)] || [];
        const labels = values.map((v: string) => {
            const found = items.find((it: any) => it.value === v || it.name === v);
            return found?.name || v;
        });

        fields.push({
            key: t.key,
            label: t.name,
            value: labels.join("、"),
            fieldName,
            typeId: String(t.id),
            rawValues: values,
            isMulti: MULTI_SELECT_KEYS.has(t.key)
        });
    });

    return fields;
});

// ===== 跟进方式格式化 =====
function formatFollowupType(v: string): string {
    const map: Record<string, string> = {
        "1": "待跟进",
        "2": "电话访谈",
        "21": "电话-无人接听",
        "22": "电话-拒接",
        "23": "电话-已接通",
        "3": "微信沟通",
        "31": "微信-待通过",
        "32": "微信-拒绝通过",
        "33": "微信-已通过",
        "4": "视频参观",
        "5": "预约参观",
        "6": "已参观"
    };
    return map[v] || v;
}

// ===== 方法 =====
const toggleCollapse = () => {
    collapsed.value = !collapsed.value;
};

const togglePersonalCollapse = () => {
    personalCollapsed.value = !personalCollapsed.value;
};

// 拨打电话（远程控制同账号 Android 客户端自动拨号+录音）
const handleCall = (event?: MouseEvent) => {
    const mobile = cluesInfo.value?.mobile;
    if (!mobile) {
        ElMessage.warning("该线索没有手机号");
        return;
    }
    // 校验客户端是否在线
    if (deviceState.connStatus !== "connected" || !deviceState.androidOnline || !deviceState.canRemoteCall) {
        ElMessage.warning("客户端未连接或未开启电脑控制手机外呼");
        return;
    }
    // 下发拨号指令（cluesId 用于移动端关联录音与跟进）
    emit("remote-call", { ...cluesInfo.value, mobile }, event);
};

const handleTransfer = async () => {
    transferForm.projectId = "";
    transferForm.groupId = "";
    transferForm.servicesId = "";
    transferGroupList.value = [];
    transferKfList.value = [];
    try {
        transferProjectList.value = await service.customer_pro.project.list();
    } catch (e) {
        console.error("获取项目列表失败:", e);
    }
    transferVisible.value = true;
};

// 转交相关
const transferVisible = ref(false);
const transferSaving = ref(false);
const transferProjectList = ref<any[]>([]);
const transferGroupList = ref<any[]>([]);
const transferKfList = ref<any[]>([]);
const transferForm = reactive({
    projectId: "",
    groupId: "",
    servicesId: ""
});

const onTransferProjectChange = async (v: string) => {
    transferForm.groupId = "";
    transferForm.servicesId = "";
    transferKfList.value = [];
    try {
        transferGroupList.value = await service.customer_pro.project_group.list({ projectId: v });
    } catch (e) {
        console.error("获取客服组列表失败:", e);
    }
};

const onTransferGroupChange = async (v: string) => {
    transferForm.servicesId = "";
    try {
        transferKfList.value = await service.customer_pro.kf.list({ groupId: v, projectId: transferForm.projectId });
    } catch (e) {
        console.error("获取客服人员列表失败:", e);
    }
};

const onTransferSubmit = async () => {
    if (!transferForm.servicesId) {
        ElMessage.warning("请选择接收人");
        return;
    }
    transferSaving.value = true;
    try {
        await service.customer_pro.clues.distribute({
            ids: [props.cluesId],
            servicesId: transferForm.servicesId
        });
        ElMessage.success("转交完成");
        transferVisible.value = false;
        loadData();
    } catch (err: any) {
        ElMessage.error(err.message || "转交失败");
    } finally {
        transferSaving.value = false;
    }
};

const emit = defineEmits(["toggleFullscreen", "close", "tagTypesChanged", "remote-call"]);

const isFullscreen = ref(false);
const fullscreenSide = ref<"left" | "right">("left");
const avatarPreviewVisible = ref(false);
const refreshingAll = ref(false);

// 同时刷新线索资料及所有已挂载的业务子模块，所有详情入口均可使用。
const refreshAllData = async () => {
    if (refreshingAll.value) return;
    refreshingAll.value = true;
    try {
        await Promise.allSettled([
            loadData(),
            Promise.resolve(followListRef.value?.refresh?.()),
            Promise.resolve(trackRef.value?.refresh?.()),
            Promise.resolve(chatRef.value?.refresh?.()),
            Promise.resolve(orderListRef.value?.refresh?.())
        ]);
        ElMessage.success("数据已刷新");
    } finally {
        refreshingAll.value = false;
    }
};

const toggleFullscreen = () => {
    fullscreenSide.value = "left";
    isFullscreen.value = !isFullscreen.value;
    emit("toggleFullscreen");
};

const toggleRightFullscreen = () => {
    fullscreenSide.value = "right";
    isFullscreen.value = !isFullscreen.value;
    emit("toggleFullscreen");
};

const emitFullscreen = () => {
    isFullscreen.value = false;
    emit("toggleFullscreen");
};

const closeDrawer = () => {
    emit("close");
};

// ===== 认领（公海模式） =====
const handleClaim = () => {
    service.customer_pro.clues
        .claimClues({ cluesId: props.cluesId })
        .then(() => {
            ElMessage.success("认领成功");
            emit("close");
        })
        .catch((e: any) => {
            ElMessage.error(e.message || "认领失败");
        });
};

// ===== 放入公海（私海模式） =====
const hasPushCommonClausePermission = computed(() => {
    return (service.customer_pro.clues as any)?._permission?.pushCommonClause || false;
});

const pushCommonClause = () => {
    (service.customer_pro.clues as any).pushCommonClause({ cluesId: props.cluesId }).then(() => {
        ElMessage.success("放入公海成功");
        emit("close");
    }).catch((e: any) => {
        ElMessage.error(e.message || "放入公海失败");
    });
};

// ===== 编辑保存回调 =====
const onEditSaved = () => {
    loadData();
    trackRef.value?.refresh();
};

// ===== 成交录入 =====
const openOrderAdd = () => {
    // 先切换到订单标签页，确保组件渲染后再触发新增
    leftTab.value = "order";
    nextTick(() => {
        orderListRef.value?.addRow();
    });
};

// 订单操作成功回调（新增/编辑/删除）
const onOrderSuccess = () => {
    // 重新加载线索数据（更新成交次数）
    loadData();
};

// ===== 聊天记录引用 =====
const chatRef = ref<InstanceType<typeof SubChat>>();
const hasChatContentListPermission = computed(() => {
    return (service.customer_pro.clues as any)?._permission?.chatContentList || false;
});

// ===== 标签管理保存后的回调 =====
async function onTagManagerSaved() {
    // 重置字典元数据缓存，强制重新加载（标签类型可能增删了）
    resetLocalDictMeta();
    await loadDictMeta();
    // 重新加载线索数据
    await loadData();
    // 通知父组件（clues.vue）刷新表格列等
    emit("tagTypesChanged");
}

// ===== 加载线索详情数据 =====
const loadData = async () => {
    if (!props.cluesId) return;
    try {
        // 并行加载字典元数据和名称映射
        loadDictMeta();
        loadNameMaps();
        await getUserInfo();

        // 获取线索详情
        cluesInfo.value = await service.customer_pro.clues.info({ id: props.cluesId });

        // console.log("cluesInfo", cluesInfo.value);

        // 加载客服列表（用于解析负责人名称、头像及参与人信息）
        try {
            const list = await service.customer_pro.kf.list({
                projectId: cluesInfo.value.projectId
            });
            kfList.value = list || [];
        } catch (e) {
            console.error("获取客服列表失败:", e);
        }

        // 跟进记录和聊天记录由子组件自行加载
    } catch (e) {
        console.error("加载线索详情失败:", e);
    }
};

onMounted(() => {
    loadData();
});

defineExpose({
    refreshFollow: () => {
        followListRef.value?.refresh();
        loadData();
    }
});
</script>

<style lang="scss" scoped>
.clue-detail {
    display: flex;
    flex-direction: column;
    gap: 12px;
    background: #f5f6f7;
    height: 100%;
    overflow: hidden;
    position: relative;

}

.detail-actions {
    position: absolute;
    top: 20px;
    right: 16px;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 4px;
}

.detail-action-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    padding: 0;
    border: 0;
    border-radius: 6px;
    background: transparent;
    color: #909399;
    cursor: pointer;
    transition: color 0.2s, background-color 0.2s;

    &:hover {
        color: var(--color-primary);
        background: var(--el-color-primary-light-9);
    }

    &:disabled {
        cursor: default;
    }

    &.is-refreshing .el-icon {
        animation: detail-refresh-spin 0.8s linear infinite;
    }
}

@keyframes detail-refresh-spin {
    to { transform: rotate(360deg); }
}

/* ========== 头像点击 ========== */
.clickable-avatar {
    cursor: pointer;
    transition: transform 0.2s;
}
.clickable-avatar:hover {
    transform: scale(1.05);
}

/* ========== 顶部卡片 ========== */
.clue-info-card {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 14px;
    padding: 30px 20px 12px;
    background: #fff;
    border-radius: 10px;

    .card-left {
        flex-shrink: 0;
        margin-top: 2px;
    }

    .card-right {
        flex: 1;
        min-width: 200px;
    }

    .row-top {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 8px;

        .name {
            font-size: 17px;
            font-weight: bold;
            color: #1a1a1a;
        }

        :deep(.el-button) {
            padding: 4px 12px;
            height: 28px;
            font-size: 13px;

            .el-icon {
                margin-right: 6px;
            }
        }
    }

    .row-mid {
        display: flex;
        align-items: center;
        gap: 4px;
        margin-bottom: 8px;
        flex-wrap: wrap;

        .label {
            color: #666;
            font-size: 13px;
        }

        .val {
            color: #333;
            font-size: 13px;
            margin-right: 2px;

            &.primary {
                color: var(--color-primary);
            }
        }
    }

    .row-tags {
        display: flex;
        align-items: center;
        gap: 8px;
        flex-wrap: wrap;
        width: 100%;
        min-height: 32px;
        padding: 4px 6px;
        border-radius: 4px;

        :deep(.el-tag) {
            font-size: 13px;
            padding: 3px 14px;
            border-radius: 4px !important;
            height: auto;
            line-height: 1.6;
        }
    }
}

/*
 * 该按钮不使用 Element Plus 默认的浅绿色 hover/focus/active 状态。
 * 二次点击关闭卡片后鼠标仍停在按钮上，必须覆盖 hover 变量及实际样式，
 * 才能立即恢复并保持原来的高亮绿色。
 */
:deep(.detail-call-anchor.el-button--success) {
    --el-button-hover-text-color: var(--el-color-white);
    --el-button-hover-bg-color: var(--el-color-success);
    --el-button-hover-border-color: var(--el-color-success);
    --el-button-active-text-color: var(--el-color-white);
    --el-button-active-bg-color: var(--el-color-success);
    --el-button-active-border-color: var(--el-color-success);
}

:deep(.detail-call-anchor.el-button--success:hover),
:deep(.detail-call-anchor.el-button--success:focus),
:deep(.detail-call-anchor.el-button--success:focus-visible),
:deep(.detail-call-anchor.el-button--success:active) {
    color: var(--el-color-white) !important;
    background-color: var(--el-color-success) !important;
    border-color: var(--el-color-success) !important;
    box-shadow: none !important;
}

/* ========== 下半部分 ========== */
.detail-body {
    display: flex;
    gap: 8px;
    flex: 1;
    min-height: 0;

    &.fullscreen-left {
        .panel-right {
            display: none;
        }

        .panel-left {
            flex: 0 0 100%;
            max-width: 100%;
        }
    }

    &.fullscreen-right {
        .panel-left {
            display: none;
        }

        .panel-right {
            flex: 0 0 100%;
            max-width: 100%;
            padding-top: 20px;
        }
    }

    &.fullscreen-left {
        .panel-left {
            padding-top: 20px;
        }
    }
}

.panel-left,
.panel-right {
    flex: 1;
    background: #fff;
    border-radius: 10px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.panel-left {
    overflow: hidden;
}

.panel-right {
    overflow: hidden;
}

.section-divider {
    height: 8px;
    background: #f5f6f7;
    margin: 12px -16px;
}

.panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 16px 12px;
    flex-shrink: 0;

    .tab-group {
        :deep(.el-radio-button__inner) {
            border: none;
            padding: 6px 18px;
            font-size: 13px;
        }

        :deep(.el-radio-button:first-child .el-radio-button__inner),
        :deep(.el-radio-button:last-child .el-radio-button__inner) {
            border-radius: 0;
        }
    }

    .header-right {
        display: flex;
        gap: 8px;
        cursor: pointer;
        color: #999;
    }
}

/* 左侧子标签栏 */
.sub-tab-bar {
    display: flex;
    align-items: center;
    gap: 20px;
    padding: 10px 16px;
    font-size: 13px;
    color: #666;
    cursor: pointer;
    flex-shrink: 0;

    .hide-empty-switch {
        margin-left: auto;
        display: flex;
        align-items: center;
        gap: 4px;

        .question-icon {
            color: #999;
            cursor: pointer;

            &:hover {
                color: var(--color-primary);
            }
        }
    }

    span.active {
        color: var(--color-primary);
    }

    .collapse-icon {
        margin-right: 4px;
        vertical-align: middle;
        cursor: pointer !important;

        &:hover {
            color: var(--color-primary);
        }
    }

    .setting-icon {
        margin-left: 8px;
        color: #ccc;
        vertical-align: middle;

        &:hover {
            color: var(--color-primary);
        }
    }
}

/* 面板内容 */
.panel-content {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-gutter: stable;
    padding: 0px 16px 16px;
}

.followup-panel {
    display: flex;
    flex-direction: column;
    gap: 12px;
    overflow-x: hidden;

    .quick-follow {
        border: 1px solid #e4e7ed;
        border-radius: 8px;
        padding: 8px 12px;
        background: #fff;

        &.expanded {
            border-color: #409eff;
            box-shadow: 0 0 0 1px #409eff inset;
        }

        :deep(.el-textarea__inner) {
            border: none;
            box-shadow: none;
            padding: 0;
            resize: none;
            min-height: 32px;
            font-size: 14px;
            transition: min-height 0.15s ease;
        }

        &.expanded :deep(.el-textarea__inner) {
            min-height: 80px !important;
        }

        .quick-follow-footer {
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 0;
            padding-top: 0;
            border-top: 1px solid transparent;
            max-height: 0;
            overflow: hidden;
            opacity: 0;
            transition: all 0.2s ease;

            &.visible {
                margin-top: 8px;
                padding-top: 8px;
                border-top-color: #f0f0f0;
                max-height: 60px;
                opacity: 1;
            }

            .footer-left {
                .next-follow-picker {
                    width: 200px;

                    :deep(.el-input__wrapper) {
                        box-shadow: none;
                        padding: 0 4px;
                    }

                    :deep(.el-input__inner) {
                        font-size: 13px;
                        color: #909399;

                        &::placeholder {
                            color: #909399;
                        }
                    }
                }
            }

            .footer-right {
                flex-shrink: 0;
            }
        }
    }
}

/* 折叠占位 */
.collapse-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 0;
    cursor: pointer;
    color: #ccc;

    .el-icon {
        transition: transform 0.2s, color 0.2s;

        &.icon-double-down {
            transform: rotate(90deg);
        }

        &:hover {
            color: var(--color-primary);
        }

        &.icon-double-down:hover {
            transform: rotate(90deg) scale(1.1);
        }
    }
}

/* 客户资料网格 */
.info-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;

    .info-row {
        display: flex;
        align-items: center;
        font-size: 13px;
        line-height: 2;

        .info-left {
            display: flex;
            align-items: center;
            gap: 4px;
            flex: 1;
            min-width: 0;

            >label {
                color: #999;
                flex-shrink: 0;
                width: 120px;
                text-align: right;
                display: inline-block;
            }
        }

        .info-right {
            display: flex;
            align-items: center;
            gap: 4px;
            flex: 1;
            min-width: 0;

            >label {
                display: inline-block;
                flex-shrink: 0;
                width: 120px;
                text-align: right;
                color: #999;
            }
        }

        span {
            color: #333;
            display: inline-flex;
            align-items: center;
            gap: 4px;

            &.link-text {
                color: var(--color-primary);
                cursor: pointer;
            }

            &.val-red {
                color: #f56c6c;
            }
        }

        a {
            color: var(--color-primary);
            text-decoration: none;
            margin: 0 3px;
        }

        .icon-gray {
            color: #ccc;
        }

        .icon-warn {
            color: #e6a23c;
        }

        .tag-edit-icon {
            color: #909399;
            cursor: pointer;
            margin-left: 4px;
            vertical-align: middle;
            transition: color 0.2s;

            &:hover {
                color: var(--color-primary);
            }
        }

        .action-icon {
            color: #fff;
            background: var(--color-primary);
            border-radius: 50%;
            width: 20px;
            height: 20px;
            cursor: pointer;
            transition: opacity 0.2s;

            &:hover {
                opacity: 0.8;
            }
        }
    }
}

/* 右侧 - 快速写跟进 */
.quick-input {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 10px 16px;
    flex-shrink: 0;

    :deep(.el-input__wrapper) {
        box-shadow: none;
        background: #f7f7f7;
        border-radius: 6px;
    }

    .add-icon {
        color: #ccc;
        font-size: 18px;
        cursor: pointer;

        &:hover {
            color: var(--color-primary);
        }
    }
}

/* 跟进时间线 */
.timeline-list {
    display: flex;
    flex-direction: column;
    gap: 14px;
    padding: 8px 0 12px;
}

.timeline-item {
    padding-bottom: 14px;
    border-bottom: 1px solid #f5f5f5;

    &:last-child {
        border-bottom: none;
        padding-bottom: 0;
    }

    .timeline-head {
        display: flex;
        align-items: center;
        gap: 6px;
        margin-bottom: 4px;

        .user-name {
            font-size: 13px;
            font-weight: 500;
            color: #333;
        }

        .time {
            font-size: 12px;
            color: #d83b01;
        }
    }

    .timeline-body {
        font-size: 13px;
        color: #666;
        padding-left: 36px;

        .contact-type {
            color: #999;
            margin-bottom: 2px;
        }

        p {
            margin: 0;
            line-height: 1.6;
        }
    }

    .timeline-footer {
        padding-left: 36px;
        margin-top: 6px;
        font-size: 12px;
        color: #999;

        a {
            color: #999;
            text-decoration: none;
            margin-right: 16px;
            display: inline-flex;
            align-items: center;
            gap: 4px;

            &:hover {
                color: var(--color-primary);
            }
        }

        .more-link {
            color: #ccc;
            cursor: pointer;
        }
    }
}
</style>
