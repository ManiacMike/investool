import re

with open('api/fund.go', 'r', encoding='utf-8') as f:
    content = f.read()

# remove 'package api' inside the file except the first one
def replace_nth(string, old, new, n):
    idx = 0
    for _ in range(n):
        idx = string.find(old, idx)
        if idx == -1: return string
        idx += len(old)
    return string[:idx - len(old)] + new + string[idx:]

content = content.replace("package api\n", "")
content = "package api\n" + content

# Fix imports
import_blocks = re.findall(r'import \(([\s\S]*?)\)', content)
all_imports = set()
for block in import_blocks:
    for line in block.split('\n'):
        line = line.strip()
        if line:
            all_imports.add(line)

content = re.sub(r'import \([\s\S]*?\)', '', content)
content = re.sub(r'import "[^"]+"', '', content)

new_imports = "import (\n" + "\n".join(["\t" + imp for imp in all_imports]) + "\n)\n"
content = content.replace("package api\n", "package api\n\n" + new_imports)

with open('api/fund.go', 'w', encoding='utf-8') as f:
    f.write(content)
