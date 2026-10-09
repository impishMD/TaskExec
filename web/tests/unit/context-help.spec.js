import './setup';
import { expect } from 'chai';
import { mount, createLocalVue } from '@vue/test-utils';
import Vuetify from 'vuetify';
import HelpContext from '@/components/HelpContext';
import HelpToggle from '@/components/HelpToggle.vue';
import HelpHint from '@/components/HelpHint.vue';

describe('Context help', () => {
  it('reveals field icons on request, opens their text on click and hides them on toggle', async () => {
    const wrapper = mount({
      mixins: [HelpContext],
      components: { HelpToggle, HelpHint },
      data: () => ({ parentEscapes: 0 }),
      template: '<div @keydown.esc="parentEscapes += 1">'
        + '<HelpToggle /><HelpHint>Field explanation</HelpHint></div>',
    }, { localVue: createLocalVue(), vuetify: new Vuetify(), mocks: { $t: (key) => key } });
    try {
      const hint = wrapper.findComponent(HelpHint);
      expect(hint.find('button').exists()).to.equal(false);
      await wrapper.findComponent(HelpToggle).find('button').trigger('click');
      expect(hint.find('button').exists()).to.equal(true);
      await hint.find('button').trigger('mouseenter');
      expect(hint.vm.open).to.equal(false);
      await hint.find('button').trigger('click');
      expect(hint.vm.open).to.equal(true);
      await hint.find('button').trigger('keydown', { key: 'Escape', keyCode: 27 });
      expect(wrapper.vm.parentEscapes).to.equal(0);
      expect(hint.vm.open).to.equal(false);
      await hint.find('button').trigger('keydown', { key: 'Escape', keyCode: 27 });
      expect(wrapper.vm.parentEscapes).to.equal(1);
      await hint.find('button').trigger('click');
      expect(hint.vm.open).to.equal(true);
      await wrapper.findComponent(HelpToggle).find('button').trigger('click');
      expect(hint.vm.open).to.equal(false);
      expect(hint.find('button').exists()).to.equal(false);
    } finally { wrapper.destroy(); }
  });
});
