package booker

import "time"

// DefaultCheckToBookDelay is the pause after a successful GolfCheckTeeTimeStatus
// (Action=0 selects the flight) before GolfNewBooking2. Booking sooner returns
// ErrNumber 20017 ("select your flight again"). 3s is the lowest delay that
// booked in probes; 2.5s did not.
const DefaultCheckToBookDelay = 3 * time.Second
