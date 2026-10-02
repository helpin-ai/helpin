-- Upgrade the previous team preset palette to solid colors.
UPDATE workspace_teams
SET color = CASE color
    WHEN '#a6ade6' THEN '#5e6ad2'
    WHEN '#9ec1f3' THEN '#4e8fea'
    WHEN '#94d2e7' THEN '#3daed4'
    WHEN '#8ccfc1' THEN '#2da88e'
    WHEN '#99cea3' THEN '#45a557'
    WHEN '#b8ce97' THEN '#7da642'
    WHEN '#e0ce94' THEN '#c7a53d'
    WHEN '#f1c093' THEN '#e58c3a'
    WHEN '#efa29b' THEN '#e2564a'
    WHEN '#f19eb5' THEN '#e54e78'
    WHEN '#e79dcb' THEN '#d44ca0'
    WHEN '#d69ee1' THEN '#b44ec9'
    WHEN '#bfa5fa' THEN '#8b5cf6'
    WHEN '#9bcae8' THEN '#4a9ed6'
    WHEN '#cbb9a8' THEN '#a08060'
    WHEN '#c1c9d3' THEN '#788596'
    ELSE color
END
WHERE color IN ('#a6ade6', '#9ec1f3', '#94d2e7', '#8ccfc1', '#99cea3', '#b8ce97', '#e0ce94', '#f1c093', '#efa29b', '#f19eb5', '#e79dcb', '#d69ee1', '#bfa5fa', '#9bcae8', '#cbb9a8', '#c1c9d3');
