import { FormRules } from 'naive-ui'
import { createNodeValidator } from '@workflowView/lib/NodeValidator'

const validator = createNodeValidator('等待')

export default {
	duration: validator.custom((rule, value) => {
		if (value === undefined || value === null || value === '') {
			return new Error('请输入等待时间')
		}
		const num = Number(value)
		if (!Number.isFinite(num) || num <= 0) {
			return new Error('等待时间必须大于0')
		}
		return true
	}, 'input'),
	unit: validator.custom((rule, value) => {
		if (!value) {
			return new Error('请选择时间单位')
		}
		return true
	}),
} as FormRules
