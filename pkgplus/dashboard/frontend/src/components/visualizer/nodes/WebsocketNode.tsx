import { type ComponentType } from 'react'

import type { WebSocket } from '@/types'
import type { NodeProps } from 'reactflow'
import NodeBase, { type NodeBaseData } from './NodeBase'

export type WebsocketNodeData = NodeBaseData<WebSocket>

export const WebsocketNode: ComponentType<NodeProps<WebsocketNodeData>> = (
  props,
) => {
  const { data } = props

  return (
    <NodeBase
      {...props}
      drawerOptions={{
        title: `Websocket - ${data.title}`,
        description: data.description,
        icon: data.icon,
        nodeType: 'websocket',
        testHref: `/websockets`, // TODO add url param to switch to resource
        address: `ws://${data.address}`,
        services: data.resource.requestingServices,
      }}
    />
  )
}
