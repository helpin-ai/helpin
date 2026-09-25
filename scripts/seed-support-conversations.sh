#!/bin/bash
# Seed dummy support conversations for UI testing
# Usage: ./seed-support-conversations.sh <API_BASE_URL> <AUTH_TOKEN>
#
# Get your auth token from browser DevTools:
#   localStorage.getItem('access_token')
#
# Example:
#   ./scripts/seed-support-conversations.sh http://localhost:8080 "eyJhbG..."

set -e

API="${1:?Usage: $0 <API_BASE_URL> <AUTH_TOKEN>}"
TOKEN="${2:?Provide your auth token (from localStorage.getItem('access_token'))}"
AUTH="Authorization: Bearer $TOKEN"

# Get workspace ID from slug
echo "Looking up workspace 'test-docs'..."
WS=$(curl -s -H "$AUTH" "$API/api/workspaces/by-slug/test-docs")
WS_ID=$(echo "$WS" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null)
if [ -z "$WS_ID" ]; then
  echo "Failed to find workspace. Response: $WS"
  exit 1
fi
echo "Workspace ID: $WS_ID"

create_conversation() {
  local subject="$1" customer_name="$2" customer_email="$3"
  local resp=$(curl -s -X POST "$API/api/support/inbox/conversations" \
    -H "$AUTH" \
    -H "Content-Type: application/json" \
    -d "{
      \"workspace_id\": \"$WS_ID\",
      \"subject\": \"$subject\",
      \"customer_name\": \"$customer_name\",
      \"customer_email\": \"$customer_email\",
      \"source\": \"chat\",
      \"priority\": \"medium\"
    }")
  echo "$resp" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])" 2>/dev/null
}

send_message() {
  local conv_id="$1" content="$2" is_internal="${3:-false}"
  curl -s -X POST "$API/api/support/inbox/conversations/$conv_id/messages" \
    -H "$AUTH" \
    -H "Content-Type: application/json" \
    -d "{\"content\": \"$content\", \"is_internal\": $is_internal}" > /dev/null
}

echo ""
echo "Creating conversations..."

# Conversation 1: Product question
C1=$(create_conversation "How do I set up team permissions?" "Sarah Chen" "sarah@example.com")
if [ -n "$C1" ]; then
  echo "  Created: How do I set up team permissions? ($C1)"
  send_message "$C1" "Hi there! I just signed up and I'm trying to figure out how to set up permissions for my team. We have about 15 people and I need different access levels for managers vs regular members. Can you help?"
  send_message "$C1" "Sure, I'd be happy to help! You can manage team permissions from **Settings > Team Members**. There you can assign roles like Owner, Admin, Member, or Viewer. Each role has different access levels." false
  send_message "$C1" "That sounds great. What's the difference between Admin and Member roles specifically?"
  sleep 0.3
fi

# Conversation 2: Bug report
C2=$(create_conversation "Dashboard charts not loading on mobile" "Marcus Johnson" "marcus@acmecorp.com")
if [ -n "$C2" ]; then
  echo "  Created: Dashboard charts not loading on mobile ($C2)"
  send_message "$C2" "The analytics dashboard charts aren't rendering on my iPhone. I'm using Safari on iOS 17. The page loads but the chart areas are just blank white boxes. This started happening yesterday."
  send_message "$C2" "Thanks for reporting this, Marcus. Let me check with the team. Could you try clearing your Safari cache and let me know if that helps?" false
  send_message "$C2" "Just tried clearing cache - same issue. Here's what I see: the loading spinner appears briefly, then disappears and the area stays blank."
  send_message "$C2" "I've reproduced this on our end. It looks like a CSS rendering issue with the chart library on WebKit. I'm escalating to engineering now." false
  sleep 0.3
fi

# Conversation 3: Feature request
C3=$(create_conversation "Can we get Slack integration?" "Emily Rodriguez" "emily@startupxyz.io")
if [ -n "$C3" ]; then
  echo "  Created: Can we get Slack integration? ($C3)"
  send_message "$C3" "Hey! Love the product so far. One thing that would be amazing is a Slack integration so we can get notifications about new support tickets directly in our team channel. Is that on the roadmap?"
  sleep 0.3
fi

# Conversation 4: Billing question
C4=$(create_conversation "Need to upgrade our plan" "David Park" "david@bigclient.com")
if [ -n "$C4" ]; then
  echo "  Created: Need to upgrade our plan ($C4)"
  send_message "$C4" "We're hitting the user limit on our current plan. Can you walk me through upgrading? We'd need to go from 10 to 50 seats."
  send_message "$C4" "Of course! You can upgrade directly from **Settings > Billing**. For 50 seats, our Business plan would be the best fit at \$29/seat/month." false
  send_message "$C4" "Sounds good. Can we get annual billing? And is there a discount for non-profits? We're a registered 501(c)(3)."
  send_message "$C4" "Yes, annual billing gives you 20% off! And we do offer non-profit discounts. Let me connect you with our billing team for the specifics." false
  send_message "$C4" "Perfect, thank you so much!"
  sleep 0.3
fi

# Conversation 5: Onboarding help
C5=$(create_conversation "Help importing data from Zendesk" "Lisa Thompson" "lisa@migratingco.com")
if [ -n "$C5" ]; then
  echo "  Created: Help importing data from Zendesk ($C5)"
  send_message "$C5" "We're migrating from Zendesk and need to import about 5,000 historical tickets. Is there a bulk import feature or API we can use?"
  send_message "$C5" "Welcome aboard! We have a CSV import tool and a REST API for bulk operations. For 5,000 tickets, I'd recommend the CSV import - it's the fastest approach." false
  send_message "$C5" "Great, where do I find the CSV import? And what format does it need to be in?"
  sleep 0.3
fi

# Conversation 6: Quick thank you
C6=$(create_conversation "Thanks for the quick fix!" "Alex Kim" "alex@happycustomer.com")
if [ -n "$C6" ]; then
  echo "  Created: Thanks for the quick fix! ($C6)"
  send_message "$C6" "Just wanted to say thanks - the bug fix you shipped yesterday solved our issue completely. Your support team is fantastic!"
  send_message "$C6" "That's wonderful to hear, Alex! Really glad we could get that resolved quickly. Don't hesitate to reach out if you need anything else." false
  sleep 0.3
fi

echo ""
echo "Done! Created 6 dummy conversations. Refresh your support inbox to see them."
