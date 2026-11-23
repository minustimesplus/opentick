import {describe, expect, it} from 'vitest'
import {renderMarkdown} from './markdown'
import {embeddedAttachmentIds} from './attachmentImages'

describe('renderMarkdown images', () => {
	it('renders attachment images without a src, for the editor to resolve', () => {
		expect(renderMarkdown('![image.png](attachment:12)')).toBe('<p><img class="md-img" data-attachment="12" alt="image.png"></p>')
	})

	it('renders http images and keeps emphasis out of the attributes', () => {
		expect(renderMarkdown('![my_shot_1](https://x.test/a_b_c.png)')).toBe('<p><img class="md-img" src="https://x.test/a_b_c.png" alt="my_shot_1"></p>')
	})

	it('does not load images from other schemes', () => {
		expect(renderMarkdown('![x](javascript:alert(1))')).not.toContain('<img')
	})

	it('shows an upload placeholder', () => {
		expect(renderMarkdown('![Uploading a.png…](uploading:1-1)')).toBe('<p><span class="md-img-uploading">Uploading a.png…</span></p>')
	})

	it('still renders links', () => {
		expect(renderMarkdown('[a](https://x.test)')).toContain('<a href="https://x.test"')
	})
})

describe('embeddedAttachmentIds', () => {
	it('collects attachment image ids', () => {
		expect([...embeddedAttachmentIds('a ![x](attachment:3)\n![](attachment:7) [l](attachment:9)')]).toEqual([3, 7])
	})
})
