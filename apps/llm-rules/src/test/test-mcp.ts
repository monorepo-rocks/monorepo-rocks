#!/usr/bin/env bun

import 'zx/globals'

// Test the MCP server
async function testMCPServer() {
	console.log('Testing MCP server...')

	// Just test that the server starts and shows the rules
	try {
		await $({
			timeout: '5s',
		})`echo '{"jsonrpc":"2.0","method":"exit","id":1}' | ${process.cwd()}/dist/llm-rules.cjs --dir ${process.cwd()}/../..`
		console.log('✅ Server starts successfully and finds rules')
		return true
	} catch (error) {
		console.error('❌ Server test failed:', error)
		return false
	}
}

// Test that tools are available via direct tool list
async function testToolsList() {
	console.log('Testing tools list...')

	try {
		const input = JSON.stringify({
			jsonrpc: '2.0',
			method: 'tools/list',
			id: 1,
			params: {},
		})

		await $({
			timeout: '3s',
		})`echo ${input} | ${process.cwd()}/dist/llm-rules.cjs --dir ${process.cwd()}/../..`
		console.log('✅ Tools list works')
		return true
	} catch (error) {
		console.error('❌ Tools list failed:', error)
		return false
	}
}

// Test calling a specific tool
async function testToolCall() {
	console.log('Testing tool call...')

	try {
		const input = JSON.stringify({
			jsonrpc: '2.0',
			method: 'tools/call',
			id: 1,
			params: {
				name: 'cursor_rule_project-overview',
				arguments: { include_frontmatter: false },
			},
		})

		const result = await $({
			timeout: '3s',
		})`echo ${input} | ${process.cwd()}/dist/llm-rules.cjs --dir ${process.cwd()}/../..`
		console.log('✅ Tool call works')
		console.log('Response length:', result.stdout.length, 'characters')
		return true
	} catch (error) {
		console.error('❌ Tool call failed:', error)
		return false
	}
}

// Run tests
const results = await Promise.all([testMCPServer(), testToolsList(), testToolCall()])

if (results.every((r) => r)) {
	console.log('\n🎉 All tests passed!')
} else {
	console.log('\n💥 Some tests failed')
	process.exit(1)
}
