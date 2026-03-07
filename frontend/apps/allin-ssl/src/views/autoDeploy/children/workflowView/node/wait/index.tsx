import { defineComponent, PropType, Ref } from 'vue'
import { useBaseNodeValidator } from '@workflowView/lib/BaseNodeValidator'
import rules from './verify'
import Drawer from './model'
import { useNodeHandler } from '@workflowView/lib/NodeHandler'
import type { WaitNodeConfig } from '@components/FlowChart/types'

interface NodeProps {
	node: {
		id: string
		config: WaitNodeConfig
	}
}

export default defineComponent({
	name: 'WaitNode',
	props: {
		node: {
			type: Object as PropType<{ id: string; config: WaitNodeConfig }>,
			default: () => ({ id: '', config: {} }),
		},
	},
	setup(props: NodeProps, { expose }) {
		const unitLabels: Record<string, string> = {
			second: '秒',
			minute: '分钟',
			hour: '小时',
		}

		const renderContent = (valid: boolean, config: WaitNodeConfig) => {
			if (valid && config?.duration) {
				const unitLabel = unitLabels[config.unit] || config.unit
				return `等待 ${config.duration} ${unitLabel}`
			}
			return '未配置'
		}

		const { renderNode } = useBaseNodeValidator(props, rules, renderContent)

		const { handleNodeClick } = useNodeHandler<WaitNodeConfig>()

		expose({
			handleNodeClick: (selectedNode: Ref<{ id: string; name: string; config: WaitNodeConfig }>) =>
				handleNodeClick(selectedNode, (node) => <Drawer node={node} />),
		})

		return renderNode
	},
})
