import os
import re
import glob

files = glob.glob('e:/Projects/relaxation-hub/relaxation-hub-server/internal/**/*_test.go', recursive=True)

# We want to replace
# func (m *{type}) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int) ([]repository.BookingDetailsResult, int, error)
# with
# func (m *{type}) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error)

pattern = r"func \(([^)]+)\)\s+ListAllWithDetailsPaginated\(\s*ctx\s+context\.Context,\s*limit,\s*offset\s+int\s*\)\s*\(\[\]repository\.BookingDetailsResult,\s*int,\s*error\)"
replacement = r"func (\1) ListAllWithDetailsPaginated(ctx context.Context, limit, offset int, search, status string) ([]repository.BookingDetailsResult, int, error)"

for file_path in files:
    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    new_content, count = re.subn(pattern, replacement, content)
    
    if count > 0:
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(new_content)
        print(f"Patched {count} instances in {os.path.basename(file_path)}")
