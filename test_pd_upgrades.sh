#!/bin/bash

# PD LTS versions
versions=("v6.5.0" "v7.1.0" "v7.5.0" "v8.1.0" "v8.5.0")

echo "Testing PD upgrade logic collection between all LTS versions..."

# Create output directory
mkdir -p /tmp/pd_upgrade_tests

# Test all combinations
for i in "${!versions[@]}"; do
    for j in $(seq $((i+1)) $((${#versions[@]}-1))); do
        from_version=${versions[$i]}
        to_version=${versions[$j]}
        
        echo "Testing upgrade logic from $from_version to $to_version..."
        
        # Run the upgrade logic collection
        cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck && \
        ./bin/kb-generator -pd-repo /Users/benjamin2037/Desktop/workspace/sourcecode/pd \
        -from-tag "$from_version" -to-tag "$to_version" -component pd
        
        # Copy the results to a unique file
        cp knowledge/pd/upgrade_logic.json "/tmp/pd_upgrade_tests/${from_version}_to_${to_version}_upgrade_logic.json"
        cp knowledge/pd/upgrade_script.sh "/tmp/pd_upgrade_tests/${from_version}_to_${to_version}_upgrade_script.sh"
        
        echo "  Results saved to /tmp/pd_upgrade_tests/${from_version}_to_${to_version}_upgrade_logic.json"
    done
done

echo "All tests completed. Results are in /tmp/pd_upgrade_tests/"