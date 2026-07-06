#!/bin/bash
# Wrapper so the tape can `Type` a plain command instead of a fragile
# pipe/quote combination. Value is an obviously-fake placeholder.
echo "demo-placeholder-value" | devspace env set demo-project DEMO_API_KEY
