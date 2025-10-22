# Coze API 集成使用说明

## 🎯 功能概述

本项目已集成Coze API，用于获取股票分析中缺失的评分数据，包括：
- **行业前景评分**（彼得·林奇评分体系）
- **行业龙头评分**（威廉·欧奈尔评分体系）
- **新品新概念评分**（威廉·欧奈尔评分体系）

## 🔧 配置步骤

### 1. 获取Coze PAT和Bot ID

1. 访问 [Coze国际版官网](https://www.coze.com/)
2. 注册账号并登录
3. 进入开发者设置，创建Personal Access Token (PAT)
4. 创建或选择一个Bot，获取Bot ID

### 2. 配置PAT和Bot ID

在 `config.toml` 文件中添加以下配置：

```toml
########## Coze API 相关配置
[coze]
    # Coze PAT (Personal Access Token) - 从 https://www.coze.com/ 获取
    api_key = "pat_your_personal_access_token_here"
    # Coze Bot ID - 你创建的股票分析机器人的ID
    bot_id = "your_bot_id_here"
    # API调用超时时间（秒）
    timeout = 30
    # 是否启用Coze分析（默认启用）
    enabled = true
```

**注意**：
- 请将 `pat_your_personal_access_token_here` 替换为你的实际PAT
- 请将 `your_bot_id_here` 替换为你的实际Bot ID
- PAT格式通常以 `pat_` 开头

### 3. 创建Coze Bot

使用提供的 `coze_bot_prompt.md` 文件中的prompt模板创建你的Bot：

1. 在Coze平台创建新的Bot
2. 复制 `coze_bot_prompt.md` 中的内容作为Bot的prompt
3. 确保Bot具有联网搜索功能
4. 配置function call功能

## 📊 评分规则详解

### 行业前景评分（0-10分）
- **9-10分**：新兴行业、政策大力支持、市场需求爆发式增长
- **7-8分**：成长性行业、政策支持、市场需求稳定增长
- **5-6分**：传统行业但有创新、政策中性、市场需求平稳
- **3-4分**：成熟行业、政策限制、市场需求下降
- **0-2分**：夕阳行业、政策打压、市场需求萎缩

### 行业龙头评分（0-10分）
- **9-10分**：绝对龙头、市场份额>30%、技术领先、品牌强势
- **7-8分**：细分龙头、市场份额15-30%、技术先进、品牌知名
- **5-6分**：区域龙头、市场份额5-15%、技术一般、品牌一般
- **3-4分**：跟随型企业、市场份额<5%、技术落后、品牌弱势
- **0-2分**：边缘企业、市场份额极小、技术落后、品牌无影响力

### 新品新概念评分（0-10分）
- **9-10分**：革命性新产品、颠覆性技术、全新赛道
- **7-8分**：创新产品、先进技术、新兴概念
- **5-6分**：改进产品、成熟技术、热门概念
- **3-4分**：传统产品、传统技术、一般概念
- **0-2分**：老旧产品、落后技术、过时概念

## 🚀 使用方法

### 1. 启动应用

```bash
go run main.go
```

### 2. 访问股票分析页面

打开浏览器访问：`http://localhost:8080/invest/stock-analyzer`

### 3. 查询股票

1. 输入股票名称或代码
2. 点击"查询股票数据"按钮
3. 系统会自动调用Coze API进行分析
4. 查看AI分析结果

## 📈 功能特点

### ✅ 已实现功能
- **自动API调用**：查询股票时自动调用Coze API
- **智能评分**：基于最新市场数据给出客观评分
- **详细分析**：提供评分依据和数据来源
- **前端展示**：美观的UI展示AI分析结果
- **错误处理**：API调用失败不影响主要功能

### 🔄 工作流程
1. 用户输入股票名称
2. 系统查询基础股票数据
3. 自动调用Coze API进行行业分析
4. 计算三个评分体系的分数
5. 在前端展示完整的分析结果

## 🛠️ 技术实现

### API调用流程
```go
// 1. 创建Coze客户端（国际版使用PAT）
cozeClient := coze.NewCozeClient(pat, botID)

// 2. 构建分析请求
req := coze.IndustryAnalysisRequest{
    StockName:    stock.BaseInfo.SecurityNameAbbr,
    Industry:     stock.BaseInfo.Industry,
    MarketCap:    stock.BaseInfo.TotalMarketCap / 100000000,
    MainBusiness: stock.MainBusiness,
    Concept:      stock.Concept,
}

// 3. 调用API获取分析结果
analysis, err := cozeClient.GetIndustryAnalysis(ctx, req)
```

**注意**：国际版API端点已自动配置为 `https://api.coze.com/open_api/v2/chat`

### 数据结构
```go
type IndustryAnalysisResponse struct {
    IndustryProspectScore float64 `json:"industry_prospect_score"` // 行业前景评分 0-10
    IndustryLeaderScore   float64 `json:"industry_leader_score"`   // 行业龙头评分 0-10
    NewConceptScore       float64 `json:"new_concept_score"`       // 新品新概念评分 0-10
    Analysis              string  `json:"analysis"`                // 分析说明
    DataSource            string  `json:"data_source"`             // 数据来源
}
```

## 🔍 故障排除

### 常见问题

1. **API调用失败**
   - 检查PAT是否正确（格式：pat_xxx）
   - 确认Bot ID是否正确
   - 检查网络连接（国际版需要访问coze.com）
   - 确认PAT权限是否足够

2. **评分不准确**
   - 确保Bot配置了联网搜索功能
   - 检查Bot的prompt是否完整
   - 验证function call配置

3. **前端不显示AI分析**
   - 检查浏览器控制台是否有错误
   - 确认API返回的数据格式正确

### 调试方法

1. **查看日志**
   ```bash
   # 启用调试日志
   export LOG_LEVEL=debug
   go run main.go
   ```

2. **检查API响应**
   - 查看HTTP响应头中的 `X-Coze-Analysis-Error`
   - 检查Coze API的返回状态

## 📝 注意事项

1. **API限制**：注意Coze API的调用频率限制
2. **数据准确性**：AI分析结果仅供参考，不构成投资建议
3. **成本控制**：API调用可能产生费用，请合理使用
4. **隐私保护**：确保API密钥安全，不要泄露给他人

## 🔮 未来改进

1. **缓存机制**：添加分析结果缓存，减少API调用
2. **批量分析**：支持多只股票同时分析
3. **历史对比**：保存历史分析结果进行对比
4. **自定义评分**：允许用户自定义评分规则

## 📞 技术支持

如有问题，请检查：
1. 配置文件是否正确
2. API密钥是否有效
3. Bot配置是否完整
4. 网络连接是否正常

---

**免责声明**：本工具提供的分析结果仅供参考，不构成投资建议。投资有风险，决策需谨慎。
