import os
import re
import glob

files = glob.glob('e:/Projects/relaxation-hub/relaxation-hub-server/internal/**/*_test.go', recursive=True)

mock_names_in_files = {
    'booking_service_mocks_test.go': ['MockBookingRepository'],
}

method_template = """
func (m *{}) ListAllEvents(ctx context.Context, params repository.ListAllEventsParams) ([]model.BookingEvent, int, error) {{
	return nil, 0, nil
}}
"""

for file_path in files:
    filename = os.path.basename(file_path)
    if filename in mock_names_in_files:
        with open(file_path, 'r', encoding='utf-8') as f:
            content = f.read()
            
        added = False
        for mock_name in mock_names_in_files[filename]:
            if f"type {mock_name} struct" in content:
                if f"func (m *{mock_name}) ListAllEvents" not in content:
                    content += method_template.format(mock_name)
                    added = True
                    
        if added:
            with open(file_path, 'w', encoding='utf-8') as f:
                f.write(content)
            print(f"Patched {filename}")
