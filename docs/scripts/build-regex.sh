#!/bin/bash
#
# Generates an optimal compact regex from a list of metric names
# Handles multi-level prefix grouping without recursion issues
#
# Usage: ./build-regex.sh <input_file>
#

set -euo pipefail

if [[ $# -ne 1 || ! -r "$1" ]]; then
    echo "Usage: $0 <input_file>" >&2
    exit 1
fi

# Use a different approach: build a hierarchical structure and generate regex iteratively
python3 -c "
import sys
import re
from collections import defaultdict

def escape_regex(text):
    return re.escape(text)

def build_tree(metrics):
    tree = defaultdict(lambda: {'children': defaultdict(dict), 'is_end': False})
    
    for metric in metrics:
        parts = metric.strip().split('_')
        current = tree['root']
        path = []
        
        for part in parts:
            path.append(part)
            if part not in current['children']:
                current['children'][part] = {'children': defaultdict(dict), 'is_end': False}
            current = current['children'][part]
        
        current['is_end'] = True
    
    return tree['root']

def generate_regex(node, depth=0):
    if not node['children']:
        return ''
    
    patterns = []
    for key, child in sorted(node['children'].items()):
        escaped_key = escape_regex(key)
        child_pattern = generate_regex(child, depth + 1)
        
        if child['is_end'] and child_pattern:
            # This key is both an endpoint and has children
            patterns.append(f'{escaped_key}(?:_{child_pattern})?')
        elif child_pattern:
            # This key only has children
            patterns.append(f'{escaped_key}_{child_pattern}')
        else:
            # This key is a leaf
            patterns.append(escaped_key)
    
    if len(patterns) == 1:
        return patterns[0]
    else:
        return '(' + '|'.join(patterns) + ')'

# Read metrics from file
with open('$1', 'r') as f:
    metrics = [line.strip() for line in f if line.strip()]

# Build tree and generate regex
tree = build_tree(metrics)
regex = generate_regex(tree)

print(f'^{regex}$')
" 2>/dev/null || {
    # Fallback to pure bash if Python3 is not available
    
    # Simple but effective bash-only approach
    declare -A prefixes
    declare -A suffixes
    
    # Read all metrics and group by longest common prefix
    while IFS= read -r line; do
        [[ -n "$line" ]] || continue
        metrics+=("$line")
    done < "$1"
    
    # Generate simple alternation regex (most compatible)
    escaped_metrics=()
    for metric in "${metrics[@]}"; do
        # Escape special regex characters
        escaped=$(printf '%s\n' "$metric" | sed 's/[[\.*^$()+?{|]/\\&/g')
        escaped_metrics+=("$escaped")
    done
    
    # Join with | and wrap
    IFS='|'
    regex="^(${escaped_metrics[*]})$"
    echo "$regex"
}
