import re

def update_file(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()

    # Find the import block and add "github.com/axiaoxin-com/investool/api"
    if 'github.com/axiaoxin-com/investool/api' not in content:
        content = content.replace('"github.com/gin-gonic/gin"', '  "github.com/axiaoxin-com/investool/api"\n\t"github.com/gin-gonic/gin"')

    # Replace usages
    functions = [
        "FundIndex", "StockIndex", "StockSelector", "StockChecker", "FundFilter", "FundCheck",
        "About", "Comment", "FundSimilarity", "Materials", "QueryFundByStock", "FundManagers",
        "InvestHoldingHandler", "StockAnalyzerHandler", "QueryStockDataHandler", "CalculateStockScoreHandler",
        "PositionDeviationHandler", "ZenStockHoldingHandler", "ZenStockNoteHandler", "QueryStockInfoHandler",
        "BatchQueryStockPricesHandler", "Ping"
    ]
    for func in functions:
        content = re.sub(r'\b(?<=[ ,(])' + func + r'\b(?=[ ,\)])', 'api.' + func, content)
        # Handle cases where func might be the very first argument like app.GET("/", FundIndex)
        content = re.sub(r' ' + func + r'\n', ' api.' + func + '\n', content)
        content = re.sub(r' ' + func + r'\)', ' api.' + func + ')', content)

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)

update_file('d:/go-projects/investool/routes/routes.go')
update_file('d:/go-projects/investool/routes/register.go')
