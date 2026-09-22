package service

// Fixture content for the sample workspace. Everything describes Northwind
// Outfitters, a fictional online outdoor-gear retailer. Every email address
// uses the reserved example.com domain.

type sampleCompany struct {
	key, name, domain, industry, headquarters, description string
	employees                                              int
}

type sampleContact struct {
	key, firstName, lastName, email, jobTitle, lifecycle, companyKey, location string
}

type sampleDeal struct {
	name, contactKey, companyKey, stageName string
	amount                                  float64
	closeInDays                             int
}

type sampleTask struct {
	key, name, description, taskType, priority, stateType, stateName string
	inEpic                                                           bool
	checklist                                                        []string
}

type sampleArticle struct {
	title, markdown string
}

type sampleMessage struct {
	sender     string // customer, teammate, note
	body       string
	minutesAgo int
}

type sampleConversation struct {
	subject, customerName, customerEmail, channel, status, priority string
	contactKey, companyKey, linkedTaskKey                           string
	messages                                                        []sampleMessage
}

var sampleCompanies = []sampleCompany{
	{key: "summit", name: "Summit Trail Guides", domain: "summittrail.example.com", industry: "Outdoor recreation", headquarters: "Boulder, CO", employees: 45,
		description: "Guided hiking and climbing company. Buys guide kits from Northwind every season."},
	{key: "bluewater", name: "Bluewater Paddle Club", domain: "bluewaterpaddle.example.com", industry: "Sports club", headquarters: "Portland, ME", employees: 120,
		description: "Member-run kayaking club evaluating a member discount program."},
	{key: "alpine", name: "Alpine Youth League", domain: "alpineyouth.example.com", industry: "Nonprofit", headquarters: "Bend, OR", employees: 18,
		description: "Youth outdoor education nonprofit that asked about a gear sponsorship."},
}

var sampleContacts = []sampleContact{
	{key: "maya", firstName: "Maya", lastName: "Chen", email: "maya.chen@summittrail.example.com", jobTitle: "Operations Lead", lifecycle: "customer", companyKey: "summit", location: "Boulder, CO"},
	{key: "diego", firstName: "Diego", lastName: "Alvarez", email: "diego.alvarez@summittrail.example.com", jobTitle: "Head Guide", lifecycle: "customer", companyKey: "summit", location: "Boulder, CO"},
	{key: "priya", firstName: "Priya", lastName: "Raman", email: "priya.raman@bluewaterpaddle.example.com", jobTitle: "Club Manager", lifecycle: "opportunity", companyKey: "bluewater", location: "Portland, ME"},
	{key: "sam", firstName: "Sam", lastName: "O'Neill", email: "sam.oneill@alpineyouth.example.com", jobTitle: "Program Director", lifecycle: "lead", companyKey: "alpine", location: "Bend, OR"},
	{key: "jordan", firstName: "Jordan", lastName: "Blake", email: "jordan.blake@example.com", jobTitle: "", lifecycle: "customer", location: "Denver, CO"},
}

var sampleDeals = []sampleDeal{
	{name: "Summit Trail Guides: fall guide kit order", contactKey: "maya", companyKey: "summit", stageName: "Proposal Sent", amount: 18400, closeInDays: 21},
	{name: "Bluewater Paddle Club: member discount program", contactKey: "priya", companyKey: "bluewater", stageName: "In Discussion", amount: 6500, closeInDays: 45},
}

const (
	sampleTeamName        = "Northwind Team"
	sampleEpicName        = "Spring catalog launch"
	sampleEpicDescription = "Everything needed to launch the spring catalog on the storefront: new photography, product copy, size guides, and restock planning."
	sampleHelpSpaceName   = "Northwind Help Center"
	sampleHelpSpaceSlug   = "northwind-help-center"
	sampleTaskCheckoutBug = "checkout_bug"
)

var sampleTasks = []sampleTask{
	{key: "photos", name: "Photograph new rain shell colorways", taskType: "feature", priority: "high", stateType: "started", stateName: "In Progress", inEpic: true,
		description: "Shoot the Ridgeline rain shell in the two new spring colors for the product page and catalog.",
		checklist:   []string{"Shoot slate gray", "Shoot forest green", "Retouch and upload to the image library"}},
	{key: "copy", name: "Write product copy for the Trailhead 45L pack", taskType: "feature", priority: "medium", stateType: "unstarted", stateName: "To Do", inEpic: true,
		description: "Product page copy for the Trailhead 45L: short intro, key features, capacity and fit notes."},
	{key: "sizes", name: "Update the size guide for women's hiking boots", taskType: "chore", priority: "low", stateType: "backlog", stateName: "Backlog", inEpic: true,
		description: "Customers keep asking whether to size up. Add half-size guidance and a foot-length chart."},
	{key: sampleTaskCheckoutBug, name: "Fix checkout error when a gift card covers the full order", taskType: "bug", priority: "urgent", stateType: "started", stateName: "In Review",
		description: "Checkout shows \"Payment method required\" when a gift card covers the whole total. Reported in the support inbox."},
	{key: "returns", name: "Publish the updated returns policy", taskType: "chore", priority: "medium", stateType: "done", stateName: "Done",
		description: "The return window is now 60 days. Update the help center article and the order confirmation email."},
	{key: "restock", name: "Plan restock for the Trailhead 45L pack", taskType: "feature", priority: "medium", stateType: "unstarted", stateName: "To Do",
		description: "Stock covers about three weeks at current sales. Agree reorder quantities with the supplier."},
}

var sampleArticles = []sampleArticle{
	{title: "Tracking your order", markdown: "Every order gets a tracking link by email as soon as it ships, usually within one business day.\n\n## Find your tracking link\n\n- Open the shipping confirmation email from Northwind Outfitters.\n- Or sign in and open **Account > Orders**.\n\n## My tracking has not updated\n\nCarriers can take up to 24 hours to show the first scan. If nothing changes after three business days, contact us and include your order number."},
	{title: "Returns and exchanges", markdown: "You can return or exchange unworn gear within **60 days** of delivery.\n\n## Start a return\n\n1. Sign in and open **Account > Orders**.\n2. Choose the order and select **Return or exchange**.\n3. Print the prepaid label and drop the parcel at any carrier location.\n\nRefunds go back to the original payment method within five business days of us receiving the parcel. Exchanges ship as soon as the return is scanned."},
	{title: "Caring for your waterproof jacket", markdown: "A clean jacket breathes better and stays waterproof longer.\n\n## Washing\n\n- Close all zips and wash on a gentle cycle at 30°C with a technical wash.\n- Do not use fabric softener.\n\n## Restoring water repellency\n\nTumble dry on low for 20 minutes to reactivate the coating. If water no longer beads on the surface, apply a spray-on repellent."},
}

var sampleConversations = []sampleConversation{
	{subject: "Where is my order NW-10482?", customerName: "Jordan Blake", customerEmail: "jordan.blake@example.com", channel: "widget", status: "open", priority: "medium", contactKey: "jordan",
		messages: []sampleMessage{
			{sender: "customer", body: "Hi! I ordered the Ridgeline rain shell last Tuesday (order NW-10482) and the tracking page hasn't changed since Friday. Is it lost?", minutesAgo: 95},
		}},
	{subject: "Gift card didn't apply at checkout", customerName: "Alex Rivera", customerEmail: "alex.rivera@example.com", channel: "email", status: "open", priority: "high", linkedTaskKey: sampleTaskCheckoutBug,
		messages: []sampleMessage{
			{sender: "customer", body: "I tried to pay for a headlamp with my $50 gift card but checkout keeps saying \"Payment method required\". The order is only $42.", minutesAgo: 300},
			{sender: "note", body: "Reproduced: any order fully covered by a gift card fails. Linked to the checkout bug in Projects.", minutesAgo: 280},
			{sender: "teammate", body: "Thanks for flagging this, Alex. Our team reproduced the problem and a fix is in review. I'll email you as soon as it's live, and I've added a $5 credit to your account for the trouble.", minutesAgo: 270},
		}},
	{subject: "Bulk order for our fall guide season", customerName: "Maya Chen", customerEmail: "maya.chen@summittrail.example.com", channel: "email", status: "waiting_on_customer", priority: "medium", contactKey: "maya", companyKey: "summit",
		messages: []sampleMessage{
			{sender: "customer", body: "Hi Northwind team, we're outfitting 30 guides for the fall season again. Can you send a quote for rain shells, 45L packs, and headlamps?", minutesAgo: 2880},
			{sender: "teammate", body: "Hi Maya, happy to help. Could you confirm the size split for the shells and whether you want the packs in one color? I'll send the quote as soon as I have that.", minutesAgo: 2820},
		}},
	{subject: "Can I exchange my boots for a half size up?", customerName: "Taylor Brooks", customerEmail: "taylor.brooks@example.com", channel: "widget", status: "resolved", priority: "low",
		messages: []sampleMessage{
			{sender: "customer", body: "My new hiking boots are a little tight at the toes. Can I swap them for a half size up?", minutesAgo: 4400},
			{sender: "teammate", body: "Absolutely. Unworn boots can be exchanged within 60 days. Open Account > Orders, choose Return or exchange, and pick the new size. The replacement ships as soon as your return is scanned.", minutesAgo: 4380},
			{sender: "customer", body: "Done, that was easy. Thanks!", minutesAgo: 4360},
		}},
}
