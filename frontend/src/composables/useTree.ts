import { shallowRef, markRaw } from 'vue'
import { GetRootItems, GetChildren, Search } from '../../bindings/reqost/collectionservice'

export interface FlatNode {
  id: string
  name: string
  parentId: string
  type: string
  method: string
  sortOrder: number
  hasChildren: boolean
  depth: number
  isExpanded: boolean
}

// Module-level state — one global tree for the sidebar.
const flatList = shallowRef<FlatNode[]>([])
const expandedIds = new Set<string>()

function toFlat(n: { id: string; name: string; parentId: string; type: string; method: string; sortOrder: number; hasChildren: boolean }, depth: number): FlatNode {
  return markRaw({ ...n, depth, isExpanded: false })
}

export function useTree() {
  async function loadRoot() {
    const nodes = await GetRootItems()
    expandedIds.clear()
    flatList.value = markRaw(nodes.map(n => toFlat(n, 0)))
  }

  async function toggleNode(node: FlatNode) {
    if (node.type !== 'folder') return
    const idx = flatList.value.findIndex(n => n.id === node.id)
    if (idx === -1) return

    if (expandedIds.has(node.id)) {
      // Collapse: remove all descendants from the flat list
      expandedIds.delete(node.id)
      const list = flatList.value.slice()
      list[idx] = { ...node, isExpanded: false }

      let end = idx + 1
      while (end < list.length && list[end].depth > node.depth) {
        expandedIds.delete(list[end].id)
        end++
      }
      list.splice(idx + 1, end - idx - 1)
      flatList.value = markRaw(list)
    } else {
      // Expand: fetch children and insert after this node
      expandedIds.add(node.id)
      const children = await GetChildren(node.id)
      const childNodes = children.map(c => toFlat(c, node.depth + 1))

      const list = flatList.value.slice()
      list[idx] = { ...node, isExpanded: true }
      list.splice(idx + 1, 0, ...childNodes)
      flatList.value = markRaw(list)
    }
  }

  async function searchNodes(query: string) {
    if (!query.trim()) {
      await loadRoot()
      return
    }
    const results = await Search(query)
    flatList.value = markRaw(results.map(n => toFlat(n, 0)))
  }

  // refreshNode patches an existing visible node in place (e.g. after Save
  // changes its name/method).
  function refreshNode(id: string, patch: Partial<FlatNode>) {
    const list = flatList.value.slice()
    const i = list.findIndex(n => n.id === id)
    if (i === -1) return
    list[i] = markRaw({ ...list[i], ...patch })
    flatList.value = markRaw(list)
  }

  // removeNode drops a node and any of its visible descendants from the list.
  function removeNode(id: string) {
    const list = flatList.value.slice()
    const i = list.findIndex(n => n.id === id)
    if (i === -1) return
    let end = i + 1
    while (end < list.length && list[end].depth > list[i].depth) {
      expandedIds.delete(list[end].id)
      end++
    }
    expandedIds.delete(id)
    list.splice(i, end - i)
    flatList.value = markRaw(list)
  }

  // reloadChildren re-fetches the children of parentId (empty == root). Used
  // after creating a node. Nested expansions under parentId collapse.
  async function reloadChildren(parentId: string) {
    if (!parentId) {
      await loadRoot()
      return
    }
    const idx = flatList.value.findIndex(n => n.id === parentId)
    if (idx === -1) return
    const parent = flatList.value[idx]
    const list = flatList.value.slice()

    // Remove current descendants.
    let end = idx + 1
    while (end < list.length && list[end].depth > parent.depth) {
      expandedIds.delete(list[end].id)
      end++
    }
    list.splice(idx + 1, end - idx - 1)

    // Re-fetch and insert; mark parent expanded + hasChildren.
    expandedIds.add(parentId)
    const children = await GetChildren(parentId)
    const childNodes = children.map(c => toFlat(c, parent.depth + 1))
    list[idx] = markRaw({ ...parent, isExpanded: true, hasChildren: true })
    list.splice(idx + 1, 0, ...childNodes)
    flatList.value = markRaw(list)
  }

  return { flatList, loadRoot, toggleNode, searchNodes, refreshNode, removeNode, reloadChildren }
}
