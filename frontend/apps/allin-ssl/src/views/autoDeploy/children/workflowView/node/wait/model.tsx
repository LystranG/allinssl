import { NFormItem, NInputNumber, NSelect } from 'naive-ui'
import { useForm, useFormHooks, useModalHooks } from '@baota/naive-ui/hooks'
import { useStore } from '@components/FlowChart/useStore'
import { useError } from '@baota/hooks/error'
import rules from './verify'
import type { WaitNodeConfig } from '@components/FlowChart/types'
import { deepClone } from '@baota/utils/data'

export default defineComponent({
	name: 'WaitNodeDrawer',
	props: {
		node: {
			type: Object as PropType<{ id: string; config: WaitNodeConfig }>,
			default: () => ({
				id: '',
				config: {
					duration: 60,
					unit: 'second',
				},
			}),
		},
	},
	setup(props) {
		const { updateNodeConfig, isRefreshNode } = useStore()
		const { useFormCustom } = useFormHooks()
		const { confirm } = useModalHooks()
		const { handleError } = useError()
		const param = ref(deepClone(props.node.config))

		const unitOptions = [
			{ label: '秒', value: 'second' },
			{ label: '分钟', value: 'minute' },
			{ label: '小时', value: 'hour' },
		]

		const formConfig = [
			useFormCustom(() => (
				<NFormItem label="等待时间" path="duration" showRequireMark={true}>
					<div class="flex items-center w-full gap-[.8rem]">
						<NInputNumber
							v-model:value={param.value.duration}
							min={1}
							showButton={false}
							class="flex-1"
							placeholder="请输入等待时间"
						/>
						<NSelect
							v-model:value={param.value.unit}
							options={unitOptions}
							style={{ width: '100px' }}
						/>
					</div>
				</NFormItem>
			)),
		]

		const {
			component: Form,
			data,
			example,
		} = useForm<WaitNodeConfig>({
			defaultValue: param,
			config: formConfig,
			rules,
		})

		confirm(async (close) => {
			try {
				await example.value?.validate()
				updateNodeConfig(props.node.id, data.value)
				isRefreshNode.value = props.node.id
				close()
			} catch (error) {
				handleError(error)
			}
		})

		return () => (
			<div class="wait-node-drawer">
				<Form labelPlacement="top" />
			</div>
		)
	},
})
