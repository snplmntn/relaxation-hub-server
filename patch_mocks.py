import os
import re
import glob

files = glob.glob('e:/Projects/relaxation-hub/relaxation-hub-server/internal/**/*_test.go', recursive=True)

# Find all structs that implement BookingRepository
# They usually have methods attached. For each file, we look for `type <name> struct`
# and if it fails to build due to missing ListAllEvents, we append it.

# Actually, we can just inject it at the end of every file that defines a mock repo.

mock_names_in_files = {
    'assignment_state_test.go': ['mockBookingRepoState'],
    'booking_service_change_for_test.go': ['MockBookingRepository'],
    'booking_service_accept_test.go': ['mockBookingRepoAssign'],
    'booking_service_test.go': ['MockBookingRepository'],
    'wallet_service_test.go': ['MockBookingRepository'],
    'booking_service_create_test.go': ['MockBookingRepository'],
    'booking_service_start_test.go': ['mockBookingRepoStart'],
    'booking_service_unassign_test.go': ['mockBookingRepoUnassign'],
    'booking_service_therapist_lifecycle_test.go': ['MockBookingRepository'],
    'broadcast_test.go': ['mockBookingRepoAW', 'mockRepoAccept'],
    'booking_service_timeline_test.go': ['mockBookingRepoTimeline'],
    'completion_worker_test.go': ['mockBookingRepoCW'],
    'report_test.go': ['mockBookingRepoReport'],
    'booking_service_admin_test.go': ['mockBookingRepoAdmin'],
    'booking_service_offers_test.go': ['mockRepoForOffers'],
    'assignment_worker_test.go': ['mockBookingRepoAW']
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
            # check if literally "type mock_name struct" is in the file
            # to make sure the mock is DEFINED in this file, not just used.
            if f"type {mock_name} struct" in content:
                # Append the method
                if f"func (m *{mock_name}) ListAllEvents" not in content:
                    content += method_template.format(mock_name)
                    added = True
                    
        if added:
            # We also need to ensure "context", "github.com/snplmnt/relaxation-hub-server/internal/repository", "github.com/snplmnt/relaxation-hub-server/internal/model" are imported.
            # But the file might use goimports to fix it up. We will rely on `goimports -w .` via bash.
            with open(file_path, 'w', encoding='utf-8') as f:
                f.write(content)
            print(f"Patched {filename}")
