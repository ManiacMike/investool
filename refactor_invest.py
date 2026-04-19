import os
import re
import shutil

# Files paths
invest_holding_path = 'd:/go-projects/investool/routes/invest_holding.go'
api_invest_path = 'd:/go-projects/investool/api/invest.go'
core_invest_path = 'd:/go-projects/investool/core/invest.go'

os.makedirs('d:/go-projects/investool/api', exist_ok=True)
os.makedirs('d:/go-projects/investool/core', exist_ok=True)

with open(invest_holding_path, 'r', encoding='utf-8') as f:
    content = f.read()

# Extract functions
# generateLLMPrompt
prompt_match = re.search(r'(?s)// generateLLMPrompt 生成LLM查询prompt.*?^}', content, re.MULTILINE)
# getScoreLevel
score_match = re.search(r'(?s)// getScoreLevel 根据分数返回等级.*?^}', content, re.MULTILINE)
# calculateTargetPosition
target_match = re.search(r'(?s)// calculateTargetPosition 计算目标仓位的辅助函数.*?^}', content, re.MULTILINE)
# getHKStockPrice
hk_match = re.search(r'(?s)// getHKStockPrice 获取港股价格.*?^}', content, re.MULTILINE)

# Remove extracted functions from api code
api_content = content
api_content = api_content.replace(prompt_match.group(0), "")
api_content = api_content.replace(score_match.group(0), "")
api_content = api_content.replace(target_match.group(0), "")

# We need to change package to api
api_content = api_content.replace('package routes', 'package api')

# We need to prepend 'core.' for the moved functions in api_content
api_content = re.sub(r'\bgenerateLLMPrompt\(', 'core.GenerateLLMPrompt(', api_content)
api_content = re.sub(r'\bgetScoreLevel\(', 'core.GetScoreLevel(', api_content)
api_content = re.sub(r'\bcalculateTargetPosition\(', 'core.CalculateTargetPosition(', api_content)
api_content = re.sub(r'\bgetHKStockPrice\(', 'core.GetHKStockPrice(', api_content)
api_content = api_content.replace('getHKStockPrice(c, result.Code)', 'core.GetHKStockPrice(c, result.Code)')
api_content = api_content.replace('getHKStockPrice(c, pureCode)', 'core.GetHKStockPrice(c, pureCode)')

# Create core_invest_path
core_functions = f"""package core

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"io"
	"net/http"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

{prompt_match.group(0).replace('generateLLMPrompt', 'GenerateLLMPrompt')}

{score_match.group(0).replace('getScoreLevel', 'GetScoreLevel')}

{target_match.group(0).replace('calculateTargetPosition', 'CalculateTargetPosition')}

{hk_match.group(0).replace('getHKStockPrice', 'GetHKStockPrice')}
"""

api_content = api_content.replace(hk_match.group(0), "")

with open(api_invest_path, 'w', encoding='utf-8') as f:
    f.write(api_content)
    
with open(core_invest_path, 'w', encoding='utf-8') as f:
    f.write(core_functions)

print("invest_holding.go refactored into api/invest.go and core/invest.go")
