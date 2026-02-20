import os

def clean_file(filepath):
    with open(filepath, 'r', encoding='utf-8') as f:
        lines = f.readlines()
    
    new_lines = []
    in_marker = False
    in_head = False
    in_other = False
    
    for line in lines:
        if line.startswith('<<<<<<<'):
            in_marker = True
            in_head = True
            continue
        elif line.startswith('======='):
            in_head = False
            in_other = True
            continue
        elif line.startswith('>>>>>>>'):
            in_marker = False
            in_other = False
            continue
            
        if not in_marker:
            new_lines.append(line)
        elif in_head:
            new_lines.append(line)
            
    with open(filepath, 'w', encoding='utf-8') as f:
        f.writelines(new_lines)

for root, _, files in os.walk('.'):
    for name in files:
        if name.endswith('.go'):
            f = os.path.join(root, name)
            try:
                with open(f, 'r', encoding='utf-8') as file:
                    content = file.read()
                if '<<<<<<<' in content:
                    clean_file(f)
                    print('Cleaned ' + f)
            except Exception as e:
                pass
